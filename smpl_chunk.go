package wav

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/go-audio/riff"
)

// smpl chunk is documented here:
// https://sites.google.com/site/musicgapi/technical-documents/wav-file-format#smpl

// DecodeSamplerChunk decodes a smpl chunk and put the data in Decoder.Metadata.SamplerInfo.
func DecodeSamplerChunk(d *Decoder, ch *riff.Chunk) error {
	if ch == nil {
		return errors.New("can't decode a nil chunk")
	}

	if d == nil {
		return errors.New("nil decoder")
	}

	if ch.ID == CIDSmpl {
		// read the entire chunk in memory
		buf := make([]byte, ch.Size)

		var err error
		if _, err = ch.Read(buf); err != nil {
			return fmt.Errorf("failed to read the smpl chunk - %w", err)
		}

		if d.Metadata == nil {
			d.Metadata = &Metadata{}
		}

		d.Metadata.SamplerInfo = &SamplerInfo{}

		Reader := bytes.NewReader(buf)

		scratch := make([]byte, 4)
		if _, err = Reader.Read(scratch); err != nil {
			return errors.New("failed to read the smpl Manufacturer")
		}

		copy(d.Metadata.SamplerInfo.Manufacturer[:], scratch[:4])

		if _, err = Reader.Read(scratch); err != nil {
			return errors.New("failed to read the smpl Product")
		}

		copy(d.Metadata.SamplerInfo.Product[:], scratch[:4])

		if err := binary.Read(Reader, binary.LittleEndian, &d.Metadata.SamplerInfo.SamplePeriod); err != nil {
			return fmt.Errorf("failed to read sample period: %w", err)
		}

		if err := binary.Read(Reader, binary.LittleEndian, &d.Metadata.SamplerInfo.MIDIUnityNote); err != nil {
			return fmt.Errorf("failed to read MIDI unity note: %w", err)
		}

		if err := binary.Read(Reader, binary.LittleEndian, &d.Metadata.SamplerInfo.MIDIPitchFraction); err != nil {
			return fmt.Errorf("failed to read MIDI pitch fraction: %w", err)
		}

		if err := binary.Read(Reader, binary.LittleEndian, &d.Metadata.SamplerInfo.SMPTEFormat); err != nil {
			return fmt.Errorf("failed to read SMPTE format: %w", err)
		}

		if err := binary.Read(Reader, binary.LittleEndian, &d.Metadata.SamplerInfo.SMPTEOffset); err != nil {
			return fmt.Errorf("failed to read SMPTE offset: %w", err)
		}

		if err := binary.Read(Reader, binary.LittleEndian, &d.Metadata.SamplerInfo.NumSampleLoops); err != nil {
			return fmt.Errorf("failed to read number of sample loops: %w", err)
		}

		var remaining uint32
		// sampler data
		if err := binary.Read(Reader, binary.BigEndian, &remaining); err != nil {
			return fmt.Errorf("failed to read remaining sampler data: %w", err)
		}

		if d.Metadata.SamplerInfo.NumSampleLoops > 0 {
			d.Metadata.SamplerInfo.Loops = []*SampleLoop{}
			for range uint32(d.Metadata.SamplerInfo.NumSampleLoops) {
				sl := &SampleLoop{}

				if _, err = Reader.Read(scratch); err != nil {
					return errors.New("failed to read the sample loop cue point id")
				}

				copy(sl.CuePointID[:], scratch[:4])

				err := binary.Read(Reader, binary.LittleEndian, &sl.Type)
				if err != nil {
					return fmt.Errorf("failed to read sample loop type: %w", err)
				}

				err = binary.Read(Reader, binary.LittleEndian, &sl.Start)
				if err != nil {
					return fmt.Errorf("failed to read sample loop start: %w", err)
				}

				err = binary.Read(Reader, binary.LittleEndian, &sl.End)
				if err != nil {
					return fmt.Errorf("failed to read sample loop end: %w", err)
				}

				err = binary.Read(Reader, binary.LittleEndian, &sl.Fraction)
				if err != nil {
					return fmt.Errorf("failed to read sample loop fraction: %w", err)
				}

				err = binary.Read(Reader, binary.LittleEndian, &sl.PlayCount)
				if err != nil {
					return fmt.Errorf("failed to read sample loop play count: %w", err)
				}

				d.Metadata.SamplerInfo.Loops = append(d.Metadata.SamplerInfo.Loops, sl)
			}
		}
	}

	ch.Drain()

	return nil
}
