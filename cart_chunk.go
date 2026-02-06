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
	cartVersionLen            = 4
	cartTitleLen              = 64
	cartArtistLen             = 64
	cartCutIDLen              = 64
	cartClientIDLen           = 64
	cartCategoryLen           = 64
	cartClassificationLen     = 64
	cartOutCueLen             = 64
	cartStartDateLen          = 10
	cartStartTimeLen          = 8
	cartEndDateLen            = 10
	cartEndTimeLen            = 8
	cartProducerAppIDLen      = 64
	cartProducerAppVersionLen = 64
	cartUserDefLen            = 64
	cartReservedLen           = 276
)

var (
	errCartNilChunk   = errors.New("can't decode a nil chunk")
	errCartNilDecoder = errors.New("nil decoder")
)

// DecodeCartChunk decodes a cart chunk into decoder metadata.
func DecodeCartChunk(d *Decoder, ch *riff.Chunk) error {
	if ch == nil {
		return errCartNilChunk
	}

	if d == nil {
		return errCartNilDecoder
	}

	if ch.ID != CIDCart {
		ch.Drain()
		return nil
	}

	buf := make([]byte, ch.Size)

	_, err := io.ReadFull(ch, buf)
	if err != nil {
		return fmt.Errorf("failed to read the cart chunk - %w", err)
	}

	if d.Metadata == nil {
		d.Metadata = &Metadata{}
	}

	cart := &Cart{}
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

	cart.Version = readFixedString(cartVersionLen)
	cart.Title = readFixedString(cartTitleLen)
	cart.Artist = readFixedString(cartArtistLen)
	cart.CutID = readFixedString(cartCutIDLen)
	cart.ClientID = readFixedString(cartClientIDLen)
	cart.Category = readFixedString(cartCategoryLen)
	cart.Classification = readFixedString(cartClassificationLen)
	cart.OutCue = readFixedString(cartOutCueLen)
	cart.StartDate = readFixedString(cartStartDateLen)
	cart.StartTime = readFixedString(cartStartTimeLen)
	cart.EndDate = readFixedString(cartEndDateLen)
	cart.EndTime = readFixedString(cartEndTimeLen)
	cart.ProducerAppID = readFixedString(cartProducerAppIDLen)
	cart.ProducerAppVersion = readFixedString(cartProducerAppVersionLen)
	cart.UserDef = readFixedString(cartUserDefLen)
	cart.LevelReference = int32(binary.LittleEndian.Uint32(take(4)))

	for i := range len(cart.PostTimer) {
		cart.PostTimer[i] = binary.LittleEndian.Uint32(take(4))
	}

	cart.Reserved = take(cartReservedLen)

	if offset < len(buf) {
		extra := buf[offset:]
		if idx := bytes.IndexByte(extra, 0); idx >= 0 {
			cart.URL = string(extra[:idx])
			tail := bytes.TrimRight(extra[idx+1:], "\x00")
			cart.TagText = string(tail)
		} else {
			cart.URL = string(extra)
		}
	}

	ch.Drain()

	d.Metadata.Cart = cart

	return nil
}

func encodeCartChunk(cart *Cart) []byte {
	if cart == nil {
		return nil
	}

	payload := bytes.NewBuffer(make([]byte, 0, 1056+len(cart.URL)+len(cart.TagText)+2))
	writeFixedString := func(s string, n int) {
		raw := make([]byte, n)
		copy(raw, []byte(s))
		payload.Write(raw)
	}

	writeFixedString(cart.Version, cartVersionLen)
	writeFixedString(cart.Title, cartTitleLen)
	writeFixedString(cart.Artist, cartArtistLen)
	writeFixedString(cart.CutID, cartCutIDLen)
	writeFixedString(cart.ClientID, cartClientIDLen)
	writeFixedString(cart.Category, cartCategoryLen)
	writeFixedString(cart.Classification, cartClassificationLen)
	writeFixedString(cart.OutCue, cartOutCueLen)
	writeFixedString(cart.StartDate, cartStartDateLen)
	writeFixedString(cart.StartTime, cartStartTimeLen)
	writeFixedString(cart.EndDate, cartEndDateLen)
	writeFixedString(cart.EndTime, cartEndTimeLen)
	writeFixedString(cart.ProducerAppID, cartProducerAppIDLen)
	writeFixedString(cart.ProducerAppVersion, cartProducerAppVersionLen)
	writeFixedString(cart.UserDef, cartUserDefLen)

	binary.Write(payload, binary.LittleEndian, uint32(cart.LevelReference))

	for i := range len(cart.PostTimer) {
		binary.Write(payload, binary.LittleEndian, cart.PostTimer[i])
	}

	reserved := make([]byte, cartReservedLen)
	copy(reserved, cart.Reserved)
	payload.Write(reserved)

	if cart.URL != "" || cart.TagText != "" {
		payload.WriteString(cart.URL)
		payload.WriteByte(0)

		if cart.TagText != "" {
			payload.WriteString(cart.TagText)
			payload.WriteByte(0)
		}
	}

	return payload.Bytes()
}
