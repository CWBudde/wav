package wav

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/go-audio/riff"
)

var (
	// ErrCuePointIDNotFound is returned when a cue point ID cannot be read.
	ErrCuePointIDNotFound = errors.New("failed to read the cue point ID")
	// ErrDataChunkIDNotFound is returned when a data chunk ID cannot be read.
	ErrDataChunkIDNotFound = errors.New("failed to read the data chunk id")
	errCueNilChunk         = errors.New("can't decode a nil chunk")
	errCueNilDecoder       = errors.New("nil decoder")
)

// CuePoint identifies a noteworthy sample position. For ordinary PCM/IEEE WAV,
// DataChunkID is "data", ChunkStart/BlockStart are zero, and SampleOffset is a
// sample-frame offset (not a byte or interleaved-channel sample offset).
type CuePoint struct {
	ID [4]byte
	// Position is the play-list sample position; zero when no play list exists.
	Position     uint32
	DataChunkID  [4]byte
	ChunkStart   uint32
	BlockStart   uint32
	SampleOffset uint32
}

// DecodeCueChunk decodes a bounded cue payload independently of the audio
// cursor. Invalid counts/truncated rows never publish partially decoded cues.
func DecodeCueChunk(decoder *Decoder, chunk *riff.Chunk) error {
	if chunk == nil {
		return errCueNilChunk
	}

	if decoder == nil {
		return errCueNilDecoder
	}

	if chunk.ID != CIDCue {
		return nil
	}

	payload, err := metadataPayload(chunk)
	if err != nil {
		return fmt.Errorf("decode cue: %w", err)
	}

	if len(payload) < 4 {
		return fmt.Errorf("%w: decode cue: missing count", errInvalidMetadata)
	}

	count := binary.LittleEndian.Uint32(payload[:4])
	if uint64(count)*24 != uint64(len(payload)-4) {
		return fmt.Errorf("%w: decode cue: count %d does not match %d payload bytes",
			errInvalidMetadata,
			count,
			len(payload)-4)
	}

	cues := make([]*CuePoint, int(count))
	reader := bytes.NewReader(payload[4:])

	for i := range cues {
		cue := &CuePoint{}
		if err := binary.Read(reader, binary.LittleEndian, cue); err != nil {
			return fmt.Errorf("decode cue row %d: %w", i, err)
		}

		cues[i] = cue
	}

	if decoder.Metadata == nil {
		decoder.Metadata = &Metadata{}
	}

	decoder.Metadata.CuePoints = cues

	return nil
}

// EncodeCueChunk serializes cue points into a RIFF payload without writing a
// file. The returned chunk can be measured and passed to Encoder.SetRawChunks.
// An empty slice encodes a valid zero-count cue chunk; nil rows are rejected.
func EncodeCueChunk(cues []*CuePoint) (RawChunk, error) {
	if len(cues) > (MaxMetadataChunkBytes-4)/24 {
		return RawChunk{}, fmt.Errorf("%w: encode cue: too many points", errInvalidMetadata)
	}

	for i, cue := range cues {
		if cue == nil {
			return RawChunk{}, fmt.Errorf("%w: encode cue: nil row %d", errInvalidMetadata, i)
		}
	}

	buffer := bytes.NewBuffer(make([]byte, 0, 4+24*len(cues)))

	_ = binary.Write(buffer, binary.LittleEndian, uint32(len(cues)))
	for _, cue := range cues {
		if err := binary.Write(buffer, binary.LittleEndian, cue); err != nil {
			return RawChunk{}, fmt.Errorf("encode cue: %w", err)
		}
	}

	return RawChunk{ID: CIDCue, Size: uint32(buffer.Len()), Data: buffer.Bytes()}, nil
}
