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

// DecodeBroadcastChunk decodes a bext chunk into decoder metadata.
func DecodeBroadcastChunk(d *Decoder, ch *riff.Chunk) error {
	if ch == nil {
		return errors.New("can't decode a nil chunk")
	}

	if d == nil {
		return errors.New("nil decoder")
	}

	if ch.ID != CIDBext {
		ch.Drain()
		return nil
	}

	buf := make([]byte, ch.Size)
	if _, err := io.ReadFull(ch, buf); err != nil {
		return fmt.Errorf("failed to read the bext chunk - %w", err)
	}

	if d.Metadata == nil {
		d.Metadata = &Metadata{}
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

	ch.Drain()

	d.Metadata.BroadcastExtension = bext

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

	binary.Write(payload, binary.LittleEndian, timeRefLow)
	binary.Write(payload, binary.LittleEndian, timeRefHigh)
	binary.Write(payload, binary.LittleEndian, bext.Version)

	payload.Write(bext.UMID[:])

	reserved := make([]byte, bextReservedLen)
	copy(reserved, bext.Reserved)
	payload.Write(reserved)

	if bext.CodingHistory != "" {
		payload.WriteString(bext.CodingHistory)
	}

	return payload.Bytes()
}
