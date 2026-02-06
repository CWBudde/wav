package wav

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/go-audio/audio"
	"github.com/go-audio/riff"
)

var (
	// CIDList is the chunk ID for a LIST chunk.
	CIDList = [4]byte{'L', 'I', 'S', 'T'}
	// CIDSmpl is the chunk ID for a smpl chunk.
	CIDSmpl = [4]byte{'s', 'm', 'p', 'l'}
	// CIDINFO is the chunk ID for an INFO chunk.
	CIDInfo = []byte{'I', 'N', 'F', 'O'}
	// CIDCue is the chunk ID for the cue chunk.
	CIDCue = [4]byte{'c', 'u', 'e', 0x20}
	// CIDFact is the chunk ID for the fact chunk.
	CIDFact = [4]byte{'f', 'a', 'c', 't'}
	// CIDBext is the chunk ID for the broadcast extension chunk.
	CIDBext = [4]byte{'b', 'e', 'x', 't'}
	// CIDCart is the chunk ID for the cart chunk.
	CIDCart = [4]byte{'c', 'a', 'r', 't'}

	// ErrPCMDataNotFound is returned when PCM data chunk is not found.
	ErrPCMDataNotFound = errors.New("PCM data not found")
	// ErrDurationNilPointer is returned when calculating duration on a nil decoder.
	ErrDurationNilPointer = errors.New("can't calculate the duration of a nil pointer")
	// ErrUnsupportedCompressedFormat is returned when a compressed audio format
	// (e.g. GSM 6.10, TrueSpeech, Voxware) is encountered that has no decoder
	// implementation. The WAV file structure is valid but the audio codec is not
	// supported.
	ErrUnsupportedCompressedFormat = errors.New("unsupported compressed audio format")
	errNilChunkOrParser            = errors.New("nil chunk/parser pointer")
	errUnhandledByteDepth          = errors.New("unhandled byte depth")
	errUnhandledFloatBitDepth      = errors.New("unhandled float bit depth")
	errUnsupportedALawBitDepth     = errors.New("unsupported A-law bit depth")
	errUnsupportedMuLawBitDepth    = errors.New("unsupported mu-law bit depth")
	errUnsupportedWavFormat        = errors.New("unsupported wav format")
)

// Decoder handles the decoding of wav files.
type Decoder struct {
	r      io.ReadSeeker
	parser *riff.Parser
	chunks *ChunkRegistry

	NumChans   uint16
	BitDepth   uint16
	SampleRate uint32

	AvgBytesPerSec uint32
	WavAudioFormat uint16
	FmtChunk       *FmtChunk

	err             error
	PCMSize         int
	pcmDataAccessed bool
	// pcmChunk is available so we can use the LimitReader
	PCMChunk *riff.Chunk
	// Metadata for the current file
	Metadata *Metadata
	// UnknownChunks stores non-core chunks for optional round-trip writing.
	UnknownChunks []RawChunk
	// CompressedSamples stores the sample count from the fact chunk for
	// compressed formats (diagnostic/informational only).
	CompressedSamples uint32

	gsmDec            *gsmDecoder
	unknownChunkOrder int
}

// NewDecoder creates a decoder for the passed wav reader.
// Note that the reader doesn't get rewinded as the container is processed.
func NewDecoder(r io.ReadSeeker) *Decoder {
	return &Decoder{
		r:      r,
		parser: riff.New(r),
		chunks: newDefaultChunkRegistry(),
	}
}

// Seek provides access to the cursor position in the PCM data.
func (d *Decoder) Seek(offset int64, whence int) (int64, error) {
	pos, err := d.r.Seek(offset, whence)
	if err != nil {
		return 0, fmt.Errorf("failed to seek: %w", err)
	}

	return pos, nil
}

// Rewind allows the decoder to be rewound to the beginning of the PCM data.
// This is useful if you want to keep on decoding the same file in a loop.
func (d *Decoder) Rewind() error {
	_, err := d.r.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("failed to seek back to the start %w", err)
	}
	// we have to user a new parser since it's read only and can't be seeked
	d.parser = riff.New(d.r)
	d.pcmDataAccessed = false
	d.PCMChunk = nil
	d.err = nil
	d.NumChans = 0
	d.CompressedSamples = 0
	d.FmtChunk = nil
	d.gsmDec = nil

	err = d.FwdToPCM()
	if err != nil {
		return fmt.Errorf("failed to seek to the PCM data: %w", err)
	}

	return nil
}

// SampleBitDepth returns the bit depth encoding of each sample.
func (d *Decoder) SampleBitDepth() int32 {
	if d == nil {
		return 0
	}

	return int32(d.BitDepth)
}

// PCMLen returns the total number of bytes in the PCM data chunk.
func (d *Decoder) PCMLen() int64 {
	if d == nil {
		return 0
	}

	return int64(d.PCMSize)
}

// Err returns the first non-EOF error that was encountered by the Decoder.
func (d *Decoder) Err() error {
	if errors.Is(d.err, io.EOF) {
		return nil
	}

	return d.err
}

// EOF returns positively if the underlying reader reached the end of file.
func (d *Decoder) EOF() bool {
	if d == nil || errors.Is(d.err, io.EOF) {
		return true
	}

	return false
}

// IsValidFile verifies that the file is valid/readable.
func (d *Decoder) IsValidFile() bool {
	d.err = d.readHeaders()
	if d.err != nil {
		return false
	}

	if d.NumChans < 1 {
		return false
	}

	if d.BitDepth < 8 && d.WavAudioFormat != wavFormatGSM610 && !isUnsupportedCompressedFormat(d.WavAudioFormat) {
		return false
	}

	dur, err := d.Duration()
	if err != nil || dur <= 0 {
		return false
	}

	return true
}

// ReadInfo reads the underlying reader until the comm header is parsed.
// This method is safe to call multiple times.
func (d *Decoder) ReadInfo() {
	d.err = d.readHeaders()
}

// ReadMetadata parses the file for extra metadata such as the INFO list chunk.
// The entire file will be read and should be rewinded if more data must be
// accessed.
func (d *Decoder) ReadMetadata() {
	if d.Metadata != nil {
		return
	}

	d.ReadInfo()

	if d.Err() != nil {
		return
	}

	d.UnknownChunks = nil
	d.unknownChunkOrder = 0

	var (
		chunk *riff.Chunk
		err   error
	)

	seenData := d.PCMChunk != nil
	for err == nil {
		chunk, err = d.NextChunk()
		if err != nil {
			break
		}

		d.unknownChunkOrder++

		if chunk.ID == riff.DataFormatID {
			seenData = true

			chunk.Drain()

			continue
		}

		handled, handleErr := d.decodeChunkViaRegistry(chunk)
		if handleErr != nil && !errors.Is(handleErr, io.EOF) {
			d.err = handleErr
		}

		if !handled {
			d.captureUnknownChunk(chunk, !seenData)
		}
	}
}

// FwdToPCM forwards the underlying reader until the start of the PCM chunk.
// If the PCM chunk was already read, no data will be found (you need to rewind).
func (d *Decoder) FwdToPCM() error {
	if d == nil {
		return ErrPCMDataNotFound
	}

	d.err = d.readHeaders()
	if d.err != nil {
		return d.err
	}

	var chunk *riff.Chunk
	for d.err == nil {
		chunk, d.err = d.NextChunk()
		if d.err != nil {
			return d.err
		}

		if chunk.ID == riff.DataFormatID {
			d.PCMSize = chunk.Size
			d.PCMChunk = chunk

			break
		}

		handled, err := d.decodeChunkViaRegistry(chunk)
		if err != nil {
			d.err = err
			return d.err
		}

		if handled {
			continue
		}

		chunk.Drain()
	}

	if chunk == nil {
		return ErrPCMDataNotFound
	}

	d.pcmDataAccessed = true

	return nil
}

// WasPCMAccessed returns positively if the PCM data was previously accessed.
func (d *Decoder) WasPCMAccessed() bool {
	if d == nil {
		return false
	}

	return d.pcmDataAccessed
}

// FullPCMBuffer is an inefficient way to access all the PCM data contained in the
// audio container. The entire PCM data is held in memory.
// Consider using PCMBuffer() instead.
func (d *Decoder) FullPCMBuffer() (*audio.Float32Buffer, error) {
	if !d.WasPCMAccessed() {
		err := d.FwdToPCM()
		if err != nil {
			return nil, d.err
		}
	}

	if d.PCMChunk == nil {
		return nil, ErrPCMChunkNotFound
	}

	format := &audio.Format{
		NumChannels: int(d.NumChans),
		SampleRate:  int(d.SampleRate),
	}

	if d.WavAudioFormat == wavFormatGSM610 {
		return d.decodeGSMBuffer(format)
	}

	if isUnsupportedCompressedFormat(d.WavAudioFormat) {
		return nil, unsupportedCompressedFormatError(d.WavAudioFormat)
	}

	return d.decodePCMBuffer(format)
}

// PCMBuffer populates the passed PCM buffer.
func (d *Decoder) PCMBuffer(buf *audio.Float32Buffer) (n int, err error) {
	if buf == nil {
		return 0, nil
	}

	if !d.pcmDataAccessed {
		err := d.FwdToPCM()
		if err != nil {
			return 0, d.err
		}
	}

	if d.PCMChunk == nil {
		return 0, ErrPCMChunkNotFound
	}

	format := &audio.Format{
		NumChannels: int(d.NumChans),
		SampleRate:  int(d.SampleRate),
	}

	buf.SourceBitDepth = int(d.BitDepth)

	if d.WavAudioFormat == wavFormatGSM610 {
		if d.gsmDec == nil {
			d.gsmDec = newGSMDecoder(int(d.CompressedSamples))
		}

		buf.SourceBitDepth = 16
		buf.Format = format

		n, err := d.gsmDec.decodeToBuffer(d.PCMChunk.R, buf.Data)
		if err != nil {
			return n, err
		}

		return n, nil
	}

	if isUnsupportedCompressedFormat(d.WavAudioFormat) {
		return 0, unsupportedCompressedFormatError(d.WavAudioFormat)
	}

	decodeF, err := sampleDecodeFloat32Func(int(d.BitDepth), d.WavAudioFormat)
	if err != nil {
		return 0, fmt.Errorf("could not get sample decode func %w", err)
	}

	bPerSample := bytesPerSample(int(d.BitDepth))
	// populate a file buffer to avoid multiple very small reads
	// we need to cap the buffer size to not be bigger than the pcm chunk.
	size := len(buf.Data) * bPerSample
	tmpBuf := make([]byte, size)

	var tmp int

	tmp, err = d.PCMChunk.R.Read(tmpBuf)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return tmp, nil
		}

		return tmp, fmt.Errorf("failed to read PCM data: %w", err)
	}

	if tmp == 0 {
		return tmp, nil
	}

	bufR := bytes.NewReader(tmpBuf[:tmp])
	sampleBuf := make([]byte, bPerSample)

	var misaligned bool
	if tmp%bPerSample > 0 {
		misaligned = true
	}

	// Note that we populate the buffer even if the
	// size of the buffer doesn't fit an even number of frames.
	for n = 0; n < len(buf.Data); n++ {
		buf.Data[n], err = decodeF(bufR, sampleBuf)
		if err != nil {
			// the last sample isn't a full sample but just padding.
			if misaligned {
				n--
			}

			break
		}
	}

	buf.Format = format

	if errors.Is(err, io.EOF) {
		err = nil
	}

	return n, err
}

// Format returns the audio format of the decoded content.
func (d *Decoder) Format() *audio.Format {
	if d == nil {
		return nil
	}

	return &audio.Format{
		NumChannels: int(d.NumChans),
		SampleRate:  int(d.SampleRate),
	}
}

// NextChunk returns the next available chunk.
func (d *Decoder) NextChunk() (*riff.Chunk, error) {
	if d.err = d.readHeaders(); d.err != nil {
		d.err = fmt.Errorf("failed to read header - %w", d.err)
		return nil, d.err
	}

	var (
		id   [4]byte
		size uint32
	)

	id, size, d.err = d.parser.IDnSize()
	if d.err != nil {
		d.err = fmt.Errorf("error reading chunk header - %w", d.err)
		return nil, d.err
	}

	// TODO: any reason we don't use d.parser.NextChunk (riff.NextChunk) here?
	// It correctly handles the misaligned chunk.

	// TODO: copied over from riff.parser.NextChunk
	// all RIFF chunks (including WAVE "data" chunks) must be word aligned.
	// If the data uses an odd number of bytes, a padding byte with a value of zero
	// must be placed at the end of the sample data.
	// The "data" chunk header's size should not include this byte.
	if size%2 == 1 {
		size++
	}

	chnk := &riff.Chunk{
		ID:   id,
		Size: int(size),
		R:    io.LimitReader(d.r, int64(size)),
	}

	return chnk, d.err
}

// Duration returns the time duration for the current audio container.
func (d *Decoder) Duration() (time.Duration, error) {
	if d == nil || d.parser == nil {
		return 0, ErrDurationNilPointer
	}

	dur, err := d.parser.Duration()
	if err != nil {
		return 0, fmt.Errorf("failed to get duration: %w", err)
	}

	return dur, nil
}

// String implements the Stringer interface.
func (d *Decoder) String() string {
	return d.parser.String()
}

func (d *Decoder) decodeGSMBuffer(format *audio.Format) (*audio.Float32Buffer, error) {
	dec := newGSMDecoder(int(d.CompressedSamples))

	samples, err := dec.decodeAllBlocks(d.PCMChunk.R, int(d.CompressedSamples))
	if err != nil {
		return nil, err
	}

	return &audio.Float32Buffer{
		Data:           samples,
		Format:         format,
		SourceBitDepth: 16,
	}, nil
}

func (d *Decoder) decodePCMBuffer(format *audio.Format) (*audio.Float32Buffer, error) {
	buf := &audio.Float32Buffer{
		Data:           make([]float32, 4096),
		Format:         format,
		SourceBitDepth: int(d.BitDepth),
	}

	bPerSample := bytesPerSample(int(d.BitDepth))
	sampleBufData := make([]byte, bPerSample)

	decodeF, err := sampleDecodeFloat32Func(int(d.BitDepth), d.WavAudioFormat)
	if err != nil {
		return nil, fmt.Errorf("could not get sample decode func %w", err)
	}

	i := 0
	for err == nil {
		buf.Data[i], err = decodeF(d.PCMChunk, sampleBufData)
		if err != nil {
			break
		}

		i++
		if i == len(buf.Data) {
			buf.Data = append(buf.Data, make([]float32, 4096)...)
		}
	}

	buf.Data = buf.Data[:i]

	if errors.Is(err, io.EOF) {
		err = nil
	}

	return buf, err
}

// readHeaders is safe to call multiple times.
func (d *Decoder) readHeaders() error {
	if d == nil || d.NumChans > 0 {
		return nil
	}

	id, size, err := d.parser.IDnSize()
	if err != nil {
		return fmt.Errorf("failed to read chunk ID and size: %w", err)
	}

	d.parser.ID = id
	if d.parser.ID != riff.RiffID {
		return fmt.Errorf("%s - %w", d.parser.ID, riff.ErrFmtNotSupported)
	}

	d.parser.Size = size

	err = binary.Read(d.r, binary.BigEndian, &d.parser.Format)
	if err != nil {
		return fmt.Errorf("failed to read format: %w", err)
	}

	var (
		chunk       *riff.Chunk
		rewindBytes int64
	)

	for err == nil {
		chunk, err = d.parser.NextChunk()
		if err != nil {
			break
		}

		if chunk.ID == riff.FmtID {
			err := d.processFmtChunk(chunk, rewindBytes)
			if err != nil {
				return err
			}

			break
		}

		d.processNonFmtChunk(chunk, &rewindBytes)
	}

	return d.err
}

func (d *Decoder) processFmtChunk(chunk *riff.Chunk, rewindBytes int64) error {
	fmtChunk, err := decodeWavHeaderChunk(chunk, d.parser)
	if err != nil {
		return fmt.Errorf("failed to decode fmt chunk: %w", err)
	}

	d.FmtChunk = fmtChunk
	d.NumChans = d.parser.NumChannels
	d.BitDepth = d.parser.BitsPerSample
	d.SampleRate = d.parser.SampleRate
	d.WavAudioFormat = d.parser.WavAudioFormat
	d.AvgBytesPerSec = d.parser.AvgBytesPerSec

	if rewindBytes > 0 {
		d.r.Seek(-(rewindBytes + int64(chunk.Size) + 8), 1)
	}

	return nil
}

func (d *Decoder) processNonFmtChunk(chunk *riff.Chunk, rewindBytes *int64) {
	if handled, _ := d.decodeHeaderChunkViaRegistry(chunk); handled {
		*rewindBytes += int64(chunk.Size) + 8
	} else {
		// unexpected chunk order, might be a bext chunk
		*rewindBytes += int64(chunk.Size) + 8
		// drain the chunk
		io.CopyN(io.Discard, d.r, int64(chunk.Size))
	}
}

func (d *Decoder) decodeChunkViaRegistry(chunk *riff.Chunk) (bool, error) {
	if d == nil || chunk == nil {
		return false, nil
	}

	if d.chunks == nil {
		d.chunks = newDefaultChunkRegistry()
	}

	return d.chunks.Decode(d, chunk)
}

func (d *Decoder) decodeHeaderChunkViaRegistry(chunk *riff.Chunk) (bool, error) {
	if chunk == nil {
		return false, nil
	}

	switch chunk.ID {
	case CIDList, CIDSmpl, CIDBext, CIDCart:
		return d.decodeChunkViaRegistry(chunk)
	default:
		return false, nil
	}
}

func decodeWavHeaderChunk(chunk *riff.Chunk, parser *riff.Parser) (*FmtChunk, error) {
	if chunk == nil || parser == nil {
		return nil, errNilChunkOrParser
	}

	fmtChunk := &FmtChunk{}

	err := chunk.ReadLE(&fmtChunk.FormatTag)
	if err != nil {
		return nil, fmt.Errorf("failed to read wav format: %w", err)
	}

	err = chunk.ReadLE(&fmtChunk.NumChannels)
	if err != nil {
		return nil, fmt.Errorf("failed to read channels: %w", err)
	}

	err = chunk.ReadLE(&fmtChunk.SampleRate)
	if err != nil {
		return nil, fmt.Errorf("failed to read sample rate: %w", err)
	}

	err = chunk.ReadLE(&fmtChunk.AvgBytesPerSec)
	if err != nil {
		return nil, fmt.Errorf("failed to read avg bytes/sec: %w", err)
	}

	err = chunk.ReadLE(&fmtChunk.BlockAlign)
	if err != nil {
		return nil, fmt.Errorf("failed to read block align: %w", err)
	}

	err = chunk.ReadLE(&fmtChunk.BitsPerSample)
	if err != nil {
		return nil, fmt.Errorf("failed to read bit depth: %w", err)
	}

	parser.NumChannels = fmtChunk.NumChannels
	parser.SampleRate = fmtChunk.SampleRate
	parser.AvgBytesPerSec = fmtChunk.AvgBytesPerSec
	parser.BlockAlign = fmtChunk.BlockAlign
	parser.BitsPerSample = fmtChunk.BitsPerSample
	parser.WavAudioFormat = fmtChunk.FormatTag

	if chunk.Size <= 16 {
		return fmtChunk, nil
	}

	var extraSize uint16

	err = chunk.ReadLE(&extraSize)
	if err != nil {
		return nil, fmt.Errorf("failed to read fmt extension size: %w", err)
	}

	fmtChunk.ExtraData = make([]byte, extraSize)
	if extraSize > 0 {
		err := chunk.ReadLE(&fmtChunk.ExtraData)
		if err != nil {
			return nil, fmt.Errorf("failed to read fmt extension data: %w", err)
		}
	}

	if fmtChunk.FormatTag != wavFormatExtensible || extraSize < 22 {
		chunk.Drain()

		return fmtChunk, nil
	}

	ext := &FmtExtensible{}
	ext.ValidBitsPerSample = binary.LittleEndian.Uint16(fmtChunk.ExtraData[0:2])
	ext.ChannelMask = binary.LittleEndian.Uint32(fmtChunk.ExtraData[2:6])
	copy(ext.SubFormat[:], fmtChunk.ExtraData[6:22])

	if len(fmtChunk.ExtraData) > 22 {
		ext.ExtraData = append(ext.ExtraData, fmtChunk.ExtraData[22:]...)
	}

	fmtChunk.Extensible = ext
	parser.WavAudioFormat = fmtChunk.EffectiveFormatTag()

	chunk.Drain()

	return fmtChunk, nil
}

func (d *Decoder) captureUnknownChunk(chunk *riff.Chunk, beforeData bool) {
	if d == nil || chunk == nil {
		return
	}

	data, err := io.ReadAll(chunk)
	if err != nil {
		d.err = fmt.Errorf("failed to read unknown chunk %s: %w", chunk.ID, err)

		return
	}

	chunk.Drain()

	d.UnknownChunks = append(d.UnknownChunks, RawChunk{
		ID:         chunk.ID,
		Size:       uint32(len(data)),
		Data:       data,
		Order:      d.unknownChunkOrder,
		BeforeData: beforeData,
	})
}

func bytesPerSample(bitDepth int) int {
	return (bitDepth-1)/8 + 1
}

func isUnsupportedCompressedFormat(wavFormat uint16) bool {
	switch wavFormat {
	case 34, 6172:
		return true
	default:
		return false
	}
}

func unsupportedCompressedFormatError(wavFormat uint16) error {
	var name string

	switch wavFormat {
	case 34:
		name = "TrueSpeech"
	case 6172:
		name = "Voxware"
	default:
		name = fmt.Sprintf("format tag %d", wavFormat)
	}

	return fmt.Errorf("%w: %s (format tag %d)", ErrUnsupportedCompressedFormat, name, wavFormat)
}

// sampleDecodeFunc returns a function that can be used to convert
// a byte range into an int value based on the amount of bits used per sample.
// Note that 8bit samples are unsigned, all other values are signed.
func sampleDecodeFunc(bitsPerSample int) (func(io.Reader, []byte) (int, error), error) {
	// NOTE: WAV PCM data is stored using little-endian
	switch {
	case bitsPerSample == 8:
		// 8bit values are unsigned
		return func(r io.Reader, buf []byte) (int, error) {
			_, err := r.Read(buf[:1])
			return int(buf[0]), err
		}, nil
	case bitsPerSample > 8 && bitsPerSample <= 16:
		return func(r io.Reader, buf []byte) (int, error) {
			_, err := r.Read(buf[:2])
			return int(int16(binary.LittleEndian.Uint16(buf[:2]))), err
		}, nil
	case bitsPerSample > 16 && bitsPerSample <= 24:
		// -34,359,738,367 (0x7FFFFF) to 34,359,738,368	(0x800000)
		return func(r io.Reader, buf []byte) (int, error) {
			_, err := r.Read(buf[:3])
			if err != nil {
				return 0, fmt.Errorf("failed to read 24-bit sample: %w", err)
			}

			return int(audio.Int24LETo32(buf[:3])), nil
		}, nil
	case bitsPerSample > 24 && bitsPerSample <= 32:
		return func(r io.Reader, buf []byte) (int, error) {
			_, err := r.Read(buf[:4])
			return int(int32(binary.LittleEndian.Uint32(buf[:4]))), err
		}, nil
	default:
		return nil, fmt.Errorf("%w: %d", errUnhandledByteDepth, bitsPerSample)
	}
}

// sampleDecodeFloat32Func returns a function that can be used to convert
// a byte range into a normalized float32 value.
func sampleDecodeFloat32Func(bitsPerSample int, wavFormat uint16) (func(io.Reader, []byte) (float32, error), error) {
	if wavFormat == wavFormatIEEEFloat {
		switch bitsPerSample {
		case 32:
			return func(r io.Reader, buf []byte) (float32, error) {
				_, err := r.Read(buf[:4])
				if err != nil {
					return 0, fmt.Errorf("failed to read 32-bit float sample: %w", err)
				}

				value := math.Float32frombits(binary.LittleEndian.Uint32(buf[:4]))

				return clampFloat32(value, -1, 1), nil
			}, nil
		case 64:
			return func(r io.Reader, buf []byte) (float32, error) {
				_, err := r.Read(buf[:8])
				if err != nil {
					return 0, fmt.Errorf("failed to read 64-bit float sample: %w", err)
				}

				value := math.Float64frombits(binary.LittleEndian.Uint64(buf[:8]))

				return clampFloat32(float32(value), -1, 1), nil
			}, nil
		default:
			return nil, fmt.Errorf("%w: %d", errUnhandledFloatBitDepth, bitsPerSample)
		}
	}

	if wavFormat == wavFormatALaw {
		if bitsPerSample != 8 {
			return nil, fmt.Errorf("%w: %d", errUnsupportedALawBitDepth, bitsPerSample)
		}

		return func(r io.Reader, buf []byte) (float32, error) {
			_, err := r.Read(buf[:1])
			if err != nil {
				return 0, fmt.Errorf("failed to read A-law sample: %w", err)
			}

			return normalizePCMInt(int(decodeALawSample(buf[0])), 16), nil
		}, nil
	}

	if wavFormat == wavFormatMuLaw {
		if bitsPerSample != 8 {
			return nil, fmt.Errorf("%w: %d", errUnsupportedMuLawBitDepth, bitsPerSample)
		}

		return func(r io.Reader, buf []byte) (float32, error) {
			_, err := r.Read(buf[:1])
			if err != nil {
				return 0, fmt.Errorf("failed to read mu-law sample: %w", err)
			}

			return normalizePCMInt(int(decodeMuLawSample(buf[0])), 16), nil
		}, nil
	}

	if isUnsupportedCompressedFormat(wavFormat) {
		return nil, unsupportedCompressedFormatError(wavFormat)
	}

	if wavFormat != wavFormatPCM {
		return nil, fmt.Errorf("%w: %d", errUnsupportedWavFormat, wavFormat)
	}

	decodeInt, err := sampleDecodeFunc(bitsPerSample)
	if err != nil {
		return nil, fmt.Errorf("failed to create int decoder: %w", err)
	}

	storageBitsPerSample := bytesPerSample(bitsPerSample) * 8

	return func(r io.Reader, buf []byte) (float32, error) {
		value, err := decodeInt(r, buf)
		if err != nil {
			return 0, fmt.Errorf("failed to decode int sample: %w", err)
		}

		return normalizePCMInt(value, storageBitsPerSample), nil
	}, nil
}
