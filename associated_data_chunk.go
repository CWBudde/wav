package wav

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/go-audio/riff"
)

// MaxMetadataChunkBytes bounds allocations from untrusted metadata chunk sizes.
// Applications may impose smaller limits before using the standalone decoders.
const MaxMetadataChunkBytes = 16 << 20

const maxMetadataBytes = 64 << 20

const (
	associatedDataType = "adtl"
	cueLabelType       = "labl"
)

var errInvalidMetadata = errors.New("invalid metadata")

func metadataPayload(chunk *riff.Chunk) ([]byte, error) {
	if chunk == nil || chunk.R == nil {
		return nil, fmt.Errorf("%w: nil metadata chunk/reader", errInvalidMetadata)
	}

	if chunk.Size < 0 || chunk.Size > MaxMetadataChunkBytes {
		return nil,
			fmt.Errorf("%w: metadata chunk size %d exceeds the %d-byte limit",
				errInvalidMetadata,
				chunk.Size,
				MaxMetadataChunkBytes)
	}

	data := make([]byte, chunk.Size)
	if _, err := io.ReadFull(chunk, data); err != nil {
		if errors.Is(err, io.EOF) {
			err = io.ErrUnexpectedEOF
		}

		return nil, fmt.Errorf("read metadata payload: %w", err)
	}

	return data, nil
}

func listSubchunks(payload []byte, visit func([4]byte, []byte) error) error {
	for offset := 0; offset < len(payload); {
		if len(payload)-offset < 8 {
			return fmt.Errorf("%w: truncated LIST subchunk header", errInvalidMetadata)
		}

		var id [4]byte
		copy(id[:], payload[offset:offset+4])
		size := uint64(binary.LittleEndian.Uint32(payload[offset+4 : offset+8]))

		remaining := uint64(len(payload) - offset - 8)
		if size > remaining || size+(size&1) > remaining {
			return fmt.Errorf("%w: truncated LIST subchunk %q payload/padding", errInvalidMetadata, id)
		}

		start := offset + 8
		if err := visit(id, payload[start:start+int(size)]); err != nil {
			return err
		}

		offset = start + int(size+(size&1))
	}

	return nil
}

// DecodeAssociatedDataChunk decodes bounded LIST/adtl metadata independently
// of the audio decoder cursor. It publishes no partial records on malformed
// input. Multiple adtl lists append their records, independently of cue order.
func DecodeAssociatedDataChunk(decoder *Decoder, chunk *riff.Chunk) error {
	if decoder == nil || chunk == nil {
		return fmt.Errorf("%w: nil associated-data decoder/chunk", errInvalidMetadata)
	}

	if chunk.ID != CIDList {
		return nil
	}

	payload, err := metadataPayload(chunk)
	if err != nil {
		return err
	}

	if len(payload) < 4 || string(payload[:4]) != associatedDataType {
		return fmt.Errorf("%w: expected LIST/adtl type", errInvalidMetadata)
	}

	return decodeAssociatedDataPayload(decoder, payload[4:])
}

func decodeAssociatedDataPayload(decoder *Decoder, payload []byte) error {
	var parsed AssociatedData

	err := listSubchunks(payload, func(id [4]byte, data []byte) error {
		switch string(id[:]) {
		case cueLabelType, "note":
			if len(data) < 4 {
				return fmt.Errorf("%w: %s record requires a cue ID", errInvalidMetadata, id)
			}

			label := CueLabel{CuePointID: binary.LittleEndian.Uint32(data[:4]), Text: nullTermStr(data[4:])}
			if string(id[:]) == cueLabelType {
				parsed.Labels = append(parsed.Labels, label)
			} else {
				parsed.Notes = append(parsed.Notes, label)
			}
		case "ltxt":
			if len(data) < 20 {
				return fmt.Errorf("%w: ltxt record requires 20 fixed bytes", errInvalidMetadata)
			}

			region := CueRegion{
				PurposeID:    [4]byte{},
				CuePointID:   binary.LittleEndian.Uint32(data[:4]),
				SampleLength: binary.LittleEndian.Uint32(data[4:8]),
				Country:      binary.LittleEndian.Uint16(data[12:14]),
				Language:     binary.LittleEndian.Uint16(data[14:16]),
				Dialect:      binary.LittleEndian.Uint16(data[16:18]),
				CodePage:     binary.LittleEndian.Uint16(data[18:20]),
				Text:         nullTermStr(data[20:]),
			}
			copy(region.PurposeID[:], data[8:12])
			parsed.Regions = append(parsed.Regions, region)
		default:
			parsed.UnknownSubchunks = append(parsed.UnknownSubchunks, RawChunk{
				ID: id, Size: uint32(len(data)), Data: append([]byte(nil), data...),
			})
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("decode adtl: %w", err)
	}

	if decoder.Metadata == nil {
		decoder.Metadata = &Metadata{}
	}

	if decoder.Metadata.AssociatedData == nil {
		decoder.Metadata.AssociatedData = new(AssociatedData)
	}

	associated := decoder.Metadata.AssociatedData
	associated.Labels = append(associated.Labels, parsed.Labels...)
	associated.Notes = append(associated.Notes, parsed.Notes...)
	associated.Regions = append(associated.Regions, parsed.Regions...)
	associated.UnknownSubchunks = append(associated.UnknownSubchunks, parsed.UnknownSubchunks...)

	return nil
}

func appendListRecord(buffer *bytes.Buffer, id [4]byte, payload []byte) error {
	if len(payload) > MaxMetadataChunkBytes || buffer.Len() > MaxMetadataChunkBytes-8-len(payload)-(len(payload)&1) {
		return fmt.Errorf("%w: encoded LIST exceeds metadata limit", errInvalidMetadata)
	}

	buffer.Write(id[:])
	_ = binary.Write(buffer, binary.LittleEndian, uint32(len(payload)))
	buffer.Write(payload)

	if len(payload)&1 != 0 {
		buffer.WriteByte(0)
	}

	return nil
}

func encodedText(text string) ([]byte, error) {
	if strings.IndexByte(text, 0) >= 0 {
		return nil, fmt.Errorf("%w: metadata text contains NUL", errInvalidMetadata)
	}

	if len(text) > MaxMetadataChunkBytes-32 {
		return nil, fmt.Errorf("%w: metadata text exceeds size limit", errInvalidMetadata)
	}

	return append([]byte(text), 0), nil
}

// EncodeAssociatedDataChunk serializes an adtl list into a bounded RIFF payload.
// The returned chunk can be measured and passed to Encoder.SetRawChunks.
func EncodeAssociatedDataChunk(a *AssociatedData) (RawChunk, error) {
	if a == nil {
		return RawChunk{}, fmt.Errorf("%w: encode adtl: nil associated data", errInvalidMetadata)
	}

	buffer := bytes.NewBufferString(associatedDataType)

	for _, group := range []struct {
		id     string
		labels []CueLabel
	}{{cueLabelType, a.Labels}, {"note", a.Notes}} {
		for _, label := range group.labels {
			text, err := encodedText(label.Text)
			if err != nil {
				return RawChunk{}, err
			}

			data := make([]byte, 4+len(text))
			binary.LittleEndian.PutUint32(data, label.CuePointID)
			copy(data[4:], text)

			var id [4]byte
			copy(id[:], group.id)

			if err := appendListRecord(buffer, id, data); err != nil {
				return RawChunk{}, err
			}
		}
	}

	for _, region := range a.Regions {
		text, err := encodedText(region.Text)
		if err != nil {
			return RawChunk{}, err
		}

		data := make([]byte, 20+len(text))
		binary.LittleEndian.PutUint32(data, region.CuePointID)
		binary.LittleEndian.PutUint32(data[4:8], region.SampleLength)
		copy(data[8:12], region.PurposeID[:])
		binary.LittleEndian.PutUint16(data[12:14], region.Country)
		binary.LittleEndian.PutUint16(data[14:16], region.Language)
		binary.LittleEndian.PutUint16(data[16:18], region.Dialect)
		binary.LittleEndian.PutUint16(data[18:20], region.CodePage)
		copy(data[20:], text)

		if err := appendListRecord(buffer, [4]byte{'l', 't', 'x', 't'}, data); err != nil {
			return RawChunk{}, err
		}
	}

	if err := appendUnknownAssociatedData(buffer, a.UnknownSubchunks); err != nil {
		return RawChunk{}, err
	}

	return RawChunk{ID: CIDList, Size: uint32(buffer.Len()), Data: buffer.Bytes()}, nil
}

func appendUnknownAssociatedData(buffer *bytes.Buffer, chunks []RawChunk) error {
	for _, chunk := range chunks {
		if err := appendListRecord(buffer, chunk.ID, chunk.Data); err != nil {
			return err
		}
	}

	return nil
}
