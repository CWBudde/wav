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

const (
	bextDescriptionLen         = 256
	bextOriginatorLen          = 32
	bextOriginatorReferenceLen = 32
	bextOriginationDateLen     = 10
	bextOriginationTimeLen     = 8
	bextUMIDLen                = 64
	bextReservedLen            = 190
)

var (
	errNilChunk   = errors.New("can't decode a nil chunk")
	errNilDecoder = errors.New("nil decoder")
)

// DecodeBroadcastChunk decodes a bext chunk into decoder metadata.
func DecodeBroadcastChunk(dec *Decoder, chnk *riff.Chunk) error {
	if chnk == nil {
		return errNilChunk
	}

	if dec == nil {
		return errNilDecoder
	}

	if chnk.ID != CIDBext {
		chnk.Drain()
		return nil
	}

	buf := make([]byte, chnk.Size)

	_, err := io.ReadFull(chnk, buf)
	if err != nil {
		return fmt.Errorf("failed to read the bext chunk - %w", err)
	}

	if dec.Metadata == nil {
		dec.Metadata = &Metadata{}
	}

	bext := &BroadcastExtension{}
	offset := 0

	take := func(n int) []byte {
		out := make([]byte, n)
		if offset < len(buf) {
			end := min(offset+n, len(buf))
			copy(out, buf[offset:end])
		}

		offset += n

		return out
	}

	readFixedString := func(n int) string {
		s := nullTermStr(take(n))
		return strings.TrimRight(s, " ")
	}

	bext.Description = readFixedString(bextDescriptionLen)
	bext.Originator = readFixedString(bextOriginatorLen)
	bext.OriginatorReference = readFixedString(bextOriginatorReferenceLen)
	bext.OriginationDate = readFixedString(bextOriginationDateLen)
	bext.OriginationTime = readFixedString(bextOriginationTimeLen)

	timeRefLow := binary.LittleEndian.Uint32(take(4))
	timeRefHigh := binary.LittleEndian.Uint32(take(4))
	bext.TimeReference = uint64(timeRefHigh)<<32 | uint64(timeRefLow)
	bext.Version = binary.LittleEndian.Uint16(take(2))

	copy(bext.UMID[:], take(bextUMIDLen))
	bext.Reserved = take(bextReservedLen)

	if offset < len(buf) {
		codingHistory := bytes.TrimRight(buf[offset:], "\x00")
		bext.CodingHistory = string(codingHistory)
	}

	chnk.Drain()

	dec.Metadata.BroadcastExtension = bext

	return nil
}

func encodeBroadcastChunk(bext *BroadcastExtension) []byte {
	if bext == nil {
		return nil
	}

	payload := bytes.NewBuffer(make([]byte, 0, 602+len(bext.CodingHistory)))
	writeFixedString := func(s string, n int) {
		raw := make([]byte, n)
		copy(raw, []byte(s))
		payload.Write(raw)
	}

	writeFixedString(bext.Description, bextDescriptionLen)
	writeFixedString(bext.Originator, bextOriginatorLen)
	writeFixedString(bext.OriginatorReference, bextOriginatorReferenceLen)
	writeFixedString(bext.OriginationDate, bextOriginationDateLen)
	writeFixedString(bext.OriginationTime, bextOriginationTimeLen)

	timeRefLow := uint32(bext.TimeReference & 0xffffffff)
	timeRefHigh := uint32((bext.TimeReference >> 32) & 0xffffffff)

	_ = binary.Write(payload, binary.LittleEndian, timeRefLow)
	_ = binary.Write(payload, binary.LittleEndian, timeRefHigh)
	_ = binary.Write(payload, binary.LittleEndian, bext.Version)

	_, _ = payload.Write(bext.UMID[:])

	reserved := make([]byte, bextReservedLen)
	copy(reserved, bext.Reserved)
	payload.Write(reserved)

	if bext.CodingHistory != "" {
		payload.WriteString(bext.CodingHistory)
	}

	return payload.Bytes()
}
