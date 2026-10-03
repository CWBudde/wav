package wav

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/go-audio/audio"
)

var (
	errIntEncoderState = errors.New("unconfigured or closed integer encoder")
	errIntGeometry     = errors.New("invalid encoder channel/sample-rate geometry")
	errIntBufferFormat = errors.New("buffer format must match encoder and contain whole frames")
	errIntSourceDepth  = errors.New("source bit depth must match encoder")
	errIntSize         = errors.New("PCM size exceeds RIFF or machine limits")
	errIntRange        = errors.New("integer sample outside signed PCM range")
)

// WriteInt writes interleaved signed PCM integer codes without converting them
// through floating point. The encoder must select PCM at 8, 16, 24 or 32 bits;
// every code must be in that depth's signed range. In particular, 8-bit caller
// codes are [-128,127] and are serialized with the WAV unsigned offset of 128.
// Format must match the encoder's channel count and sample rate, and Data must
// contain whole frames. SourceBitDepth may be zero (unspecified) or match the
// encoder's BitDepth. Empty buffers emit a valid empty data chunk.
//
// All input validation precedes header/sample writes and leaves encoder state
// unchanged on rejection. I/O failures may have written partial data. Successful
// calls consume but do not retain or modify Data, reuse the encoder's byte
// scratch, and can be interleaved with Write calls. Call Close to finish RIFF
// sizes, padding and metadata. Writes after Close are rejected.
func (e *Encoder) WriteInt(buf *audio.IntBuffer) error {
	if err := e.validateIntBuffer(buf); err != nil {
		return fmt.Errorf("wav.WriteInt: %w", err)
	}

	if err := e.startDataChunk(); err != nil {
		return fmt.Errorf("wav.WriteInt: %w", err)
	}

	e.buf.Grow(len(buf.Data) * (e.BitDepth / 8))

	var sample [4]byte

	for _, value := range buf.Data {
		switch e.BitDepth {
		case 8:
			_ = e.buf.WriteByte(byte(value + 128))
		case 16:
			binary.LittleEndian.PutUint16(sample[:2], uint16(value))
			_, _ = e.buf.Write(sample[:2])
		case 24:
			binary.LittleEndian.PutUint32(sample[:], uint32(value))
			_, _ = e.buf.Write(sample[:3])
		case 32:
			binary.LittleEndian.PutUint32(sample[:], uint32(value))
			_, _ = e.buf.Write(sample[:])
		}
	}

	written, err := e.w.Write(e.buf.Bytes())
	want := e.buf.Len()
	e.WrittenBytes += written
	e.buf.Reset()

	if err != nil {
		return fmt.Errorf("wav.WriteInt: samples: %w", err)
	}

	if written != want {
		return fmt.Errorf("wav.WriteInt: samples: %w", io.ErrShortWrite)
	}

	e.frames += len(buf.Data) / e.NumChans

	return nil
}

func (e *Encoder) validateIntBuffer(buf *audio.IntBuffer) error {
	if err := e.validateIntState(); err != nil {
		return err
	}

	if buf == nil {
		return errNilBuffer
	}

	if e.effectiveAudioFormat() != wavFormatPCM {
		return fmt.Errorf("%w: integer PCM required", errUnsupportedWavFormat)
	}

	switch e.BitDepth {
	case 8, 16, 24, 32:
	default:
		return fmt.Errorf("%w: %d", errUnsupportedFrameBitSize, e.BitDepth)
	}

	if err := e.validateIntGeometry(); err != nil {
		return err
	}

	if err := e.validateIntFormat(buf); err != nil {
		return err
	}

	return e.validateIntCodes(buf.Data)
}

func (e *Encoder) validateIntState() error {
	if e == nil {
		return errNilEncoder
	}

	if e.w == nil {
		return errNilWriter
	}

	if e.buf == nil {
		return errIntEncoderState
	}

	if e.wroteUnknownPost {
		return errIntEncoderState
	}

	return nil
}

func (e *Encoder) validateIntGeometry() error {
	align := int64(e.NumChans) * int64(e.BitDepth/8)
	if e.NumChans <= 0 || e.NumChans > math.MaxUint16 || align > math.MaxUint16 ||
		e.SampleRate <= 0 || int64(e.SampleRate) > math.MaxUint32 ||
		uint64(e.SampleRate)*uint64(align) > math.MaxUint32 {
		return errIntGeometry
	}

	return nil
}

func (e *Encoder) validateIntFormat(buf *audio.IntBuffer) error {
	if buf.Format == nil || buf.Format.NumChannels != e.NumChans ||
		buf.Format.SampleRate != e.SampleRate || len(buf.Data)%e.NumChans != 0 {
		return errIntBufferFormat
	}

	if buf.SourceBitDepth != 0 && buf.SourceBitDepth != e.BitDepth {
		return errIntSourceDepth
	}

	return nil
}

func (e *Encoder) validateIntCodes(data []int) error {
	bytes := uint64(len(data)) * uint64(e.BitDepth/8)

	maxInt := int(^uint(0) >> 1)
	if e.WrittenBytes < 0 || bytes > uint64(maxInt) ||
		uint64(e.WrittenBytes)+bytes+44 > math.MaxUint32 ||
		uint64(e.WrittenBytes)+bytes+44 > uint64(maxInt) ||
		e.frames < 0 || len(data)/e.NumChans > maxInt-e.frames {
		return errIntSize
	}

	return e.validateIntRange(data)
}

func (e *Encoder) validateIntRange(data []int) error {
	hi := int64(1) << uint(e.BitDepth-1)
	for _, value := range data {
		if int64(value) < -hi || int64(value) >= hi {
			return fmt.Errorf("%w: %d-bit", errIntRange, e.BitDepth)
		}
	}

	return nil
}
