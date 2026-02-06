package wav

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/go-audio/audio"
	"github.com/go-audio/riff"
)

// Encoder encodes LPCM data into a wav containter.
type Encoder struct {
	w   io.WriteSeeker
	buf *bytes.Buffer

	SampleRate int
	BitDepth   int
	NumChans   int

	// A number indicating the WAVE format category of the file. The content of
	// the <format-specific-fields> portion of the ‘fmt’ chunk, and the
	// interpretation of the waveform data, depend on this value. PCM = 1 (i.e.
	// Linear quantization) Values other than 1 indicate some form of
	// compression.
	WavAudioFormat int
	// FmtChunk optionally controls fmt chunk serialization, including
	// WAVE_FORMAT_EXTENSIBLE fields.
	FmtChunk *FmtChunk

	// Metadata contains metadata to inject in the file.
	Metadata *Metadata
	// UnknownChunks contains non-core chunks to preserve on write.
	UnknownChunks []RawChunk

	WrittenBytes     int
	frames           int
	pcmChunkStarted  bool
	pcmChunkSizePos  int
	wroteHeader      bool // true if we've written the header out
	wroteUnknownPre  bool
	wroteUnknownPost bool
}

// NewEncoder creates a new encoder to create a new wav file.
// Don't forget to add Frames to the encoder before writing.
func NewEncoder(w io.WriteSeeker, sampleRate, bitDepth, numChans, audioFormat int) *Encoder {
	return &Encoder{
		w:              w,
		buf:            bytes.NewBuffer(make([]byte, 0, bytesNumFromDuration(time.Minute, sampleRate, bitDepth)*numChans)),
		SampleRate:     sampleRate,
		BitDepth:       bitDepth,
		NumChans:       numChans,
		WavAudioFormat: audioFormat,
	}
}

// NewEncoderFromDecoder creates an encoder initialized from decoder settings.
// It carries format details and preserved unknown chunks for round-trip flows.
func NewEncoderFromDecoder(w io.WriteSeeker, dec *Decoder) *Encoder {
	if dec == nil {
		return NewEncoder(w, 0, 0, 0, 0)
	}

	enc := NewEncoder(w, int(dec.SampleRate), int(dec.BitDepth), int(dec.NumChans), int(dec.WavAudioFormat))
	if dec.FmtChunk != nil {
		enc.FmtChunk = dec.FmtChunk.Clone()
	}

	if len(dec.UnknownChunks) > 0 {
		enc.UnknownChunks = make([]RawChunk, len(dec.UnknownChunks))
		for i := range dec.UnknownChunks {
			enc.UnknownChunks[i] = dec.UnknownChunks[i].Clone()
		}
	}

	return enc
}

// AddLE serializes and adds the passed value using little endian.
func (e *Encoder) AddLE(src any) error {
	e.WrittenBytes += binary.Size(src)

	err := binary.Write(e.w, binary.LittleEndian, src)
	if err != nil {
		return fmt.Errorf("failed to write little endian: %w", err)
	}

	return nil
}

// AddBE serializes and adds the passed value using big endian.
func (e *Encoder) AddBE(src any) error {
	e.WrittenBytes += binary.Size(src)

	err := binary.Write(e.w, binary.BigEndian, src)
	if err != nil {
		return fmt.Errorf("failed to write big endian: %w", err)
	}

	return nil
}

var (
	errNilBuffer                   = errors.New("can't add a nil buffer")
	errAlreadyWroteHdr             = errors.New("already wrote header")
	errNilEncoder                  = errors.New("can't write a nil encoder")
	errNilWriter                   = errors.New("can't write to a nil writer")
	errEncUnsupportedFloatBitDepth = errors.New("unsupported float bit depth")
	errUnsupportedFrameBitSize     = errors.New("can't add frames of bit size")
)

func (e *Encoder) addBuffer(buf *audio.Float32Buffer) error {
	if buf == nil {
		return errNilBuffer
	}

	frameCount := buf.NumFrames()
	audioFormat := e.effectiveAudioFormat()
	// performance tweak: setup a buffer so we don't do too many writes
	var err error

	for i := range frameCount {
		for j := range buf.Format.NumChannels {
			val := buf.Data[i*buf.Format.NumChannels+j]

			if audioFormat == wavFormatIEEEFloat {
				switch e.BitDepth {
				case 32:
					err = binary.Write(e.buf, binary.LittleEndian, clampFloat32(val, -1, 1))
					if err != nil {
						return fmt.Errorf("failed to write float32 sample: %w", err)
					}
				case 64:
					err = binary.Write(e.buf, binary.LittleEndian, clampFloat64(float64(val), -1, 1))
					if err != nil {
						return fmt.Errorf("failed to write float64 sample: %w", err)
					}
				default:
					return fmt.Errorf("%w: %d", errEncUnsupportedFloatBitDepth, e.BitDepth)
				}

				continue
			}

			if audioFormat == wavFormatALaw {
				if e.BitDepth != 8 {
					return fmt.Errorf("%w: %d", errUnsupportedALawBitDepth, e.BitDepth)
				}

				err := e.buf.WriteByte(encodeALawSample(int16(float32ToPCMInt32(val, 16))))
				if err != nil {
					return fmt.Errorf("failed to write A-law sample: %w", err)
				}

				continue
			}

			if audioFormat == wavFormatMuLaw {
				if e.BitDepth != 8 {
					return fmt.Errorf("%w: %d", errUnsupportedMuLawBitDepth, e.BitDepth)
				}

				err := e.buf.WriteByte(encodeMuLawSample(int16(float32ToPCMInt32(val, 16))))
				if err != nil {
					return fmt.Errorf("failed to write mu-law sample: %w", err)
				}

				continue
			}

			if audioFormat != wavFormatPCM {
				return fmt.Errorf("%w: %d", errUnsupportedWavFormat, audioFormat)
			}

			switch e.BitDepth {
			case 8:
				err = binary.Write(e.buf, binary.LittleEndian, float32ToPCMUint8(val))
				if err != nil {
					return fmt.Errorf("failed to write 8-bit sample: %w", err)
				}
			case 16:
				err = binary.Write(e.buf, binary.LittleEndian, int16(float32ToPCMInt32(val, 16)))
				if err != nil {
					return fmt.Errorf("failed to write 16-bit sample: %w", err)
				}
			case 24:
				err = binary.Write(e.buf, binary.LittleEndian, audio.Int32toInt24LEBytes(float32ToPCMInt32(val, 24)))
				if err != nil {
					return fmt.Errorf("failed to write 24-bit sample: %w", err)
				}
			case 32:
				err = binary.Write(e.buf, binary.LittleEndian, float32ToPCMInt32(val, 32))
				if err != nil {
					return fmt.Errorf("failed to write 32-bit frame: %w", err)
				}
			default:
				return fmt.Errorf("%w: %d", errUnsupportedFrameBitSize, e.BitDepth)
			}
		}

		e.frames++
	}

	if n, err := e.w.Write(e.buf.Bytes()); err != nil {
		e.WrittenBytes += n
		return fmt.Errorf("failed to write buffer: %w", err)
	}

	e.WrittenBytes += e.buf.Len()
	e.buf.Reset()

	return nil
}

func (e *Encoder) writeHeader() error {
	if e.wroteHeader {
		return errAlreadyWroteHdr
	}

	e.wroteHeader = true
	if e == nil {
		return errNilEncoder
	}

	if e.w == nil {
		return errNilWriter
	}

	if e.WrittenBytes > 0 {
		return nil
	}

	// riff ID
	err := e.AddLE(riff.RiffID)
	if err != nil {
		return err
	}
	// file size uint32, to update later on.
	err = e.AddLE(uint32(4294967295))
	if err != nil {
		return err
	}
	// wave headers
	err = e.AddLE(riff.WavFormatID)
	if err != nil {
		return err
	}
	// form
	err = e.AddLE(riff.FmtID)
	if err != nil {
		return err
	}

	return e.writeFmtChunk()
}

// Write encodes and writes the passed buffer to the underlying writer.
// Don't forget to Close() the encoder or the file won't be valid.
func (e *Encoder) Write(buf *audio.Float32Buffer) error {
	if !e.wroteHeader {
		err := e.writeHeader()
		if err != nil {
			return err
		}
	}

	if !e.pcmChunkStarted {
		if !e.wroteUnknownPre {
			err := e.writeUnknownChunks(true)
			if err != nil {
				return fmt.Errorf("error encoding pre-data unknown chunks %w", err)
			}

			e.wroteUnknownPre = true
		}

		// sound header
		err := e.AddLE(riff.DataFormatID)
		if err != nil {
			return fmt.Errorf("error encoding sound header %w", err)
		}

		e.pcmChunkStarted = true

		// write a temporary chunksize
		e.pcmChunkSizePos = e.WrittenBytes

		err = e.AddLE(uint32(4294967295))
		if err != nil {
			return fmt.Errorf("%w when writing wav data chunk size header", err)
		}
	}

	return e.addBuffer(buf)
}

// WriteFrame writes a single frame of data to the underlying writer.
func (e *Encoder) WriteFrame(value any) error {
	if !e.wroteHeader {
		e.writeHeader()
	}

	if !e.pcmChunkStarted {
		if !e.wroteUnknownPre {
			err := e.writeUnknownChunks(true)
			if err != nil {
				return fmt.Errorf("error encoding pre-data unknown chunks %w", err)
			}

			e.wroteUnknownPre = true
		}

		// sound header
		err := e.AddLE(riff.DataFormatID)
		if err != nil {
			return fmt.Errorf("error encoding sound header %w", err)
		}

		e.pcmChunkStarted = true

		// write a temporary chunksize
		e.pcmChunkSizePos = e.WrittenBytes

		err = e.AddLE(uint32(4294967295))
		if err != nil {
			return fmt.Errorf("%w when writing wav data chunk size header", err)
		}
	}

	e.frames++

	switch val := value.(type) {
	case float32:
		audioFormat := e.effectiveAudioFormat()
		if audioFormat == wavFormatIEEEFloat {
			switch e.BitDepth {
			case 32:
				return e.AddLE(clampFloat32(val, -1, 1))
			case 64:
				return e.AddLE(clampFloat64(float64(val), -1, 1))
			default:
				return fmt.Errorf("%w: %d", errEncUnsupportedFloatBitDepth, e.BitDepth)
			}
		}

		if audioFormat == wavFormatALaw {
			if e.BitDepth != 8 {
				return fmt.Errorf("%w: %d", errUnsupportedALawBitDepth, e.BitDepth)
			}

			return e.AddLE(encodeALawSample(int16(float32ToPCMInt32(val, 16))))
		}

		if audioFormat == wavFormatMuLaw {
			if e.BitDepth != 8 {
				return fmt.Errorf("%w: %d", errUnsupportedMuLawBitDepth, e.BitDepth)
			}

			return e.AddLE(encodeMuLawSample(int16(float32ToPCMInt32(val, 16))))
		}

		if audioFormat != wavFormatPCM {
			return fmt.Errorf("%w: %d", errUnsupportedWavFormat, audioFormat)
		}

		switch e.BitDepth {
		case 8:
			return e.AddLE(float32ToPCMUint8(val))
		case 16:
			return e.AddLE(int16(float32ToPCMInt32(val, 16)))
		case 24:
			return e.AddLE(audio.Int32toInt24LEBytes(float32ToPCMInt32(val, 24)))
		case 32:
			return e.AddLE(float32ToPCMInt32(val, 32))
		default:
			return fmt.Errorf("%w: %d", errUnsupportedFrameBitSize, e.BitDepth)
		}
	case float64:
		if e.effectiveAudioFormat() == wavFormatIEEEFloat {
			switch e.BitDepth {
			case 32:
				return e.AddLE(clampFloat32(float32(val), -1, 1))
			case 64:
				return e.AddLE(clampFloat64(val, -1, 1))
			default:
				return fmt.Errorf("%w: %d", errEncUnsupportedFloatBitDepth, e.BitDepth)
			}
		}

		return e.WriteFrame(float32(val))
	default:
		return e.AddLE(value)
	}
}

func (e *Encoder) effectiveAudioFormat() int {
	if e.FmtChunk != nil {
		return int(e.FmtChunk.EffectiveFormatTag())
	}

	return e.WavAudioFormat
}

func (e *Encoder) effectiveBlockAlign() int {
	return e.NumChans * bytesPerSample(e.BitDepth)
}

func (e *Encoder) buildFmtChunkForWrite() *FmtChunk {
	blockAlign := e.effectiveBlockAlign()

	chunk := &FmtChunk{
		FormatTag:      uint16(e.WavAudioFormat),
		NumChannels:    uint16(e.NumChans),
		SampleRate:     uint32(e.SampleRate),
		AvgBytesPerSec: uint32(e.SampleRate * blockAlign),
		BlockAlign:     uint16(blockAlign),
		BitsPerSample:  uint16(e.BitDepth),
	}
	if e.FmtChunk != nil {
		chunk = e.FmtChunk.Clone()
		chunk.NumChannels = uint16(e.NumChans)
		chunk.SampleRate = uint32(e.SampleRate)
		chunk.BlockAlign = uint16(blockAlign)
		chunk.BitsPerSample = uint16(e.BitDepth)
		chunk.AvgBytesPerSec = uint32(e.SampleRate * blockAlign)
	}

	if chunk.FormatTag == wavFormatExtensible && chunk.Extensible == nil {
		chunk.Extensible = &FmtExtensible{
			ValidBitsPerSample: uint16(e.BitDepth),
			SubFormat:          makeSubFormatGUID(uint16(e.effectiveAudioFormat())),
		}
	}

	return chunk
}

func (e *Encoder) writeFmtChunk() error {
	chunk := e.buildFmtChunkForWrite()

	formatTag := chunk.FormatTag

	needsExtensible := formatTag == wavFormatExtensible && chunk.Extensible != nil
	if !needsExtensible {
		err := e.AddLE(uint32(16))
		if err != nil {
			return err
		}
	} else {
		extraLen := 22 + len(chunk.Extensible.ExtraData)

		err := e.AddLE(uint32(16 + 2 + extraLen))
		if err != nil {
			return err
		}
	}

	err := e.AddLE(formatTag)
	if err != nil {
		return err
	}

	err = e.AddLE(chunk.NumChannels)
	if err != nil {
		return fmt.Errorf("error encoding the number of channels - %w", err)
	}

	err = e.AddLE(chunk.SampleRate)
	if err != nil {
		return fmt.Errorf("error encoding the sample rate - %w", err)
	}

	err = e.AddLE(chunk.AvgBytesPerSec)
	if err != nil {
		return fmt.Errorf("error encoding the avg bytes per sec - %w", err)
	}

	err = e.AddLE(chunk.BlockAlign)
	if err != nil {
		return err
	}

	err = e.AddLE(chunk.BitsPerSample)
	if err != nil {
		return fmt.Errorf("error encoding bits per sample - %w", err)
	}

	if !needsExtensible {
		return nil
	}

	extraLen := uint16(22 + len(chunk.Extensible.ExtraData))

	err = e.AddLE(extraLen)
	if err != nil {
		return fmt.Errorf("error encoding fmt extension length - %w", err)
	}

	err = e.AddLE(chunk.Extensible.ValidBitsPerSample)
	if err != nil {
		return fmt.Errorf("error encoding valid bits per sample - %w", err)
	}

	err = e.AddLE(chunk.Extensible.ChannelMask)
	if err != nil {
		return fmt.Errorf("error encoding channel mask - %w", err)
	}

	err = e.AddLE(chunk.Extensible.SubFormat)
	if err != nil {
		return fmt.Errorf("error encoding sub format - %w", err)
	}

	if len(chunk.Extensible.ExtraData) > 0 {
		n, err := e.w.Write(chunk.Extensible.ExtraData)
		e.WrittenBytes += n

		if err != nil {
			return fmt.Errorf("error encoding extensible extra data - %w", err)
		}
	}

	return nil
}

func (e *Encoder) writeMetadata() error {
	if e == nil || e.Metadata == nil {
		return nil
	}

	if err := e.encodeMetadataViaRegistry(); err != nil {
		return err
	}

	chunkData := encodeInfoChunk(e)
	if len(chunkData) == 0 {
		return nil
	}

	err := e.AddBE(CIDList)
	if err != nil {
		return fmt.Errorf("failed to write the LIST chunk ID: %w", err)
	}

	err = e.AddLE(uint32(len(chunkData)))
	if err != nil {
		return fmt.Errorf("failed to write the LIST chunk size: %w", err)
	}

	return e.AddBE(chunkData)
}

func (e *Encoder) encodeMetadataViaRegistry() error {
	registry := newDefaultChunkRegistry()

	for _, handler := range registry.handlers {
		err := handler.Encode(e)
		if err == nil || errors.Is(err, errChunkEncodeNotSupported) {
			continue
		}

		return fmt.Errorf("failed to encode metadata chunk with %T: %w", handler, err)
	}

	return nil
}

func (e *Encoder) writeRawChunk(chunk RawChunk) error {
	size := uint32(len(chunk.Data))

	err := e.AddBE(chunk.ID)
	if err != nil {
		return fmt.Errorf("failed to write raw chunk id %q: %w", chunk.ID, err)
	}

	err = e.AddLE(size)
	if err != nil {
		return fmt.Errorf("failed to write raw chunk size %q: %w", chunk.ID, err)
	}

	if len(chunk.Data) > 0 {
		n, err := e.w.Write(chunk.Data)
		e.WrittenBytes += n

		if err != nil {
			return fmt.Errorf("failed to write raw chunk payload %q: %w", chunk.ID, err)
		}
	}

	if size%2 == 1 {
		n, err := e.w.Write([]byte{0})
		e.WrittenBytes += n

		if err != nil {
			return fmt.Errorf("failed to write raw chunk padding %q: %w", chunk.ID, err)
		}
	}

	return nil
}

func (e *Encoder) writeUnknownChunks(beforeData bool) error {
	for _, chunk := range e.UnknownChunks {
		if chunk.BeforeData != beforeData {
			continue
		}

		err := e.writeRawChunk(chunk)
		if err != nil {
			return err
		}
	}

	return nil
}

// Close flushes the content to disk, make sure the headers are up to date
// Note that the underlying writer is NOT being closed.
func (e *Encoder) Close() error {
	if e == nil || e.w == nil {
		return nil
	}

	if !e.wroteHeader && (e.Metadata != nil || len(e.UnknownChunks) > 0) {
		err := e.writeHeader()
		if err != nil {
			return err
		}
	}

	if !e.wroteUnknownPre {
		err := e.writeUnknownChunks(true)
		if err != nil {
			return fmt.Errorf("failed to write pre-data unknown chunks: %w", err)
		}

		e.wroteUnknownPre = true
	}

	if !e.wroteUnknownPost {
		err := e.writeUnknownChunks(false)
		if err != nil {
			return fmt.Errorf("failed to write post-data unknown chunks: %w", err)
		}

		e.wroteUnknownPost = true
	}

	// inject metadata at the end to not trip implementation not supporting
	// metadata chunks
	if e.Metadata != nil {
		err := e.writeMetadata()
		if err != nil {
			return fmt.Errorf("failed to write metadata - %w", err)
		}
	}

	// go back and write total size in header
	if _, err := e.w.Seek(4, 0); err != nil {
		return fmt.Errorf("failed to seek to file size position: %w", err)
	}

	err := e.AddLE(uint32(e.WrittenBytes) - 8)
	if err != nil {
		return fmt.Errorf("%w when writing the total written bytes", err)
	}

	// rewrite the audio chunk length header
	if e.pcmChunkSizePos > 0 {
		if _, err := e.w.Seek(int64(e.pcmChunkSizePos), 0); err != nil {
			return fmt.Errorf("failed to seek to PCM chunk size position: %w", err)
		}

		chunksize := uint32((e.BitDepth / 8) * e.NumChans * e.frames)

		err := e.AddLE(chunksize)
		if err != nil {
			return fmt.Errorf("%w when writing wav data chunk size header", err)
		}
	}

	// jump back to the end of the file.
	if _, err := e.w.Seek(0, 2); err != nil {
		return fmt.Errorf("failed to seek to end of file: %w", err)
	}

	if f, ok := e.w.(*os.File); ok {
		return f.Sync()
	}

	return nil
}
