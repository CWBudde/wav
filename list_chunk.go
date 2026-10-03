package wav

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/go-audio/riff"
)

var (
	// See http://bwfmetaedit.sourceforge.net/listinfo.html
	markerIART    = [4]byte{'I', 'A', 'R', 'T'}
	markerISFT    = [4]byte{'I', 'S', 'F', 'T'}
	markerICRD    = [4]byte{'I', 'C', 'R', 'D'}
	markerICOP    = [4]byte{'I', 'C', 'O', 'P'}
	markerIARL    = [4]byte{'I', 'A', 'R', 'L'}
	markerINAM    = [4]byte{'I', 'N', 'A', 'M'}
	markerIENG    = [4]byte{'I', 'E', 'N', 'G'}
	markerIGNR    = [4]byte{'I', 'G', 'N', 'R'}
	markerIPRD    = [4]byte{'I', 'P', 'R', 'D'}
	markerISRC    = [4]byte{'I', 'S', 'R', 'C'}
	markerISBJ    = [4]byte{'I', 'S', 'B', 'J'}
	markerICMT    = [4]byte{'I', 'C', 'M', 'T'}
	markerITRK    = [4]byte{'I', 'T', 'R', 'K'}
	markerITRKBug = [4]byte{'i', 't', 'r', 'k'}
	markerITCH    = [4]byte{'I', 'T', 'C', 'H'}
	markerIKEY    = [4]byte{'I', 'K', 'E', 'Y'}
	markerIMED    = [4]byte{'I', 'M', 'E', 'D'}

	errListNilChunk   = errors.New("can't decode a nil chunk")
	errListNilDecoder = errors.New("nil decoder")
)

// DecodeListChunk decodes a LIST chunk.
func DecodeListChunk(decoder *Decoder, chunk *riff.Chunk) error {
	if chunk == nil {
		return errListNilChunk
	}

	if decoder == nil {
		return errListNilDecoder
	}

	if chunk.ID != CIDList {
		chunk.Drain()

		return nil
	}

	buf, err := metadataPayload(chunk)
	if err != nil {
		return fmt.Errorf("decode LIST: %w", err)
	}

	if len(buf) < 4 {
		return fmt.Errorf("%w: decode LIST: missing list type", errInvalidMetadata)
	}

	if string(buf[:4]) == associatedDataType {
		return decodeAssociatedDataPayload(decoder, buf[4:])
	}

	if !bytes.Equal(buf[:4], CIDInfo) {
		return nil
	}

	metadata := Metadata{}
	if decoder.Metadata != nil {
		metadata = *decoder.Metadata
	}

	fields := map[[4]byte]*string{
		markerIARL:    &metadata.Location,
		markerIART:    &metadata.Artist,
		markerISFT:    &metadata.Software,
		markerICRD:    &metadata.CreationDate,
		markerICOP:    &metadata.Copyright,
		markerINAM:    &metadata.Title,
		markerIENG:    &metadata.Engineer,
		markerIGNR:    &metadata.Genre,
		markerIPRD:    &metadata.Product,
		markerISRC:    &metadata.Source,
		markerISBJ:    &metadata.Subject,
		markerICMT:    &metadata.Comments,
		markerITRK:    &metadata.TrackNbr,
		markerITRKBug: &metadata.TrackNbr,
		markerITCH:    &metadata.Technician,
		markerIKEY:    &metadata.Keywords,
		markerIMED:    &metadata.Medium,
	}

	err = listSubchunks(buf[4:], func(id [4]byte, scratch []byte) error {
		assignInfoField(fields[id], scratch)

		return nil
	})
	if err != nil {
		return fmt.Errorf("decode INFO: %w", err)
	}

	decoder.Metadata = &metadata

	chunk.Drain()

	return nil
}

func assignInfoField(field *string, payload []byte) {
	if field != nil {
		*field = nullTermStr(payload)
	}
}

func encodeInfoChunk(enc *Encoder) ([]byte, error) {
	if enc == nil || enc.Metadata == nil {
		return nil, nil
	}

	buf := bytes.NewBuffer(append([]byte(nil), CIDInfo...))

	writeSection := func(id [4]byte, val string) error {
		if val == "" {
			return nil
		}

		text, err := encodedText(val)
		if err != nil {
			return err
		}

		return appendListRecord(buf, id, text)
	}

	// Table-driven approach to reduce cyclomatic complexity
	fields := []struct {
		marker [4]byte
		value  string
	}{
		{markerIART, enc.Metadata.Artist},
		{markerICMT, enc.Metadata.Comments},
		{markerICOP, enc.Metadata.Copyright},
		{markerICRD, enc.Metadata.CreationDate},
		{markerIENG, enc.Metadata.Engineer},
		{markerITCH, enc.Metadata.Technician},
		{markerIGNR, enc.Metadata.Genre},
		{markerIKEY, enc.Metadata.Keywords},
		{markerIMED, enc.Metadata.Medium},
		{markerINAM, enc.Metadata.Title},
		{markerIPRD, enc.Metadata.Product},
		{markerISBJ, enc.Metadata.Subject},
		{markerISFT, enc.Metadata.Software},
		{markerISRC, enc.Metadata.Source},
		{markerIARL, enc.Metadata.Location},
		{markerITRK, enc.Metadata.TrackNbr},
	}

	for _, field := range fields {
		if err := writeSection(field.marker, field.value); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}
