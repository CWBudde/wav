package wav

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

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
func DecodeListChunk(d *Decoder, ch *riff.Chunk) error {
	if ch == nil {
		return errListNilChunk
	}

	if d == nil {
		return errListNilDecoder
	}

	if ch.ID == CIDList {
		// read the entire chunk in memory
		buf := make([]byte, ch.Size)

		n, err := io.ReadFull(ch, buf)
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			return fmt.Errorf("failed to read the LIST chunk - %w", err)
		}

		buf = buf[:n]

		reader := bytes.NewReader(buf)
		// INFO subchunk
		scratch := make([]byte, 4)
		if _, err = reader.Read(scratch); err != nil {
			return fmt.Errorf("failed to read the INFO subchunk - %w", err)
		}

		if !bytes.Equal(scratch, CIDInfo) {
			// "expected an INFO subchunk but got %s", string(scratch)
			// TODO: support adtl subchunks
			ch.Drain()
			return nil
		}

		if d.Metadata == nil {
			d.Metadata = &Metadata{}
		}

		// the rest is a list of string entries
		var (
			id   [4]byte
			size uint32
		)

		readSubHeader := func() error {
			err := binary.Read(reader, binary.BigEndian, &id)
			if err != nil {
				return fmt.Errorf("failed to read sub header ID: %w", err)
			}

			err = binary.Read(reader, binary.LittleEndian, &size)
			if err != nil {
				return fmt.Errorf("failed to read sub header size: %w", err)
			}

			return nil
		}

		// This checks and stops early if just a word alignment byte remains to avoid
		// an io.UnexpectedEOF error from readSubHeader.
		// TODO(steve): Remove the checks from the for statement if ch.Size is changed
		// to not include the padding byte.
		for rem := ch.Size - 4; rem > 1; rem -= int(size) + 8 {
			err = readSubHeader()
			if err != nil {
				if errors.Is(err, io.EOF) {
					// All done.
					break
				}

				return fmt.Errorf("read sub header: %w", err)
			}

			if cap(scratch) >= int(size) {
				if len(scratch) != int(size) {
					// Resize scratch.
					scratch = scratch[:size]
				}
			} else {
				// Expand scratch capacity.
				scratch = append(make([]byte, int(size)-cap(scratch)), scratch[:cap(scratch)]...)
			}

			if _, err := reader.Read(scratch); err != nil {
				return fmt.Errorf("read sub header %s data %v: %w", id, scratch, err)
			}

			switch id {
			case markerIARL:
				d.Metadata.Location = nullTermStr(scratch)
			case markerIART:
				d.Metadata.Artist = nullTermStr(scratch)
			case markerISFT:
				d.Metadata.Software = nullTermStr(scratch)
			case markerICRD:
				d.Metadata.CreationDate = nullTermStr(scratch)
			case markerICOP:
				d.Metadata.Copyright = nullTermStr(scratch)
			case markerINAM:
				d.Metadata.Title = nullTermStr(scratch)
			case markerIENG:
				d.Metadata.Engineer = nullTermStr(scratch)
			case markerIGNR:
				d.Metadata.Genre = nullTermStr(scratch)
			case markerIPRD:
				d.Metadata.Product = nullTermStr(scratch)
			case markerISRC:
				d.Metadata.Source = nullTermStr(scratch)
			case markerISBJ:
				d.Metadata.Subject = nullTermStr(scratch)
			case markerICMT:
				d.Metadata.Comments = nullTermStr(scratch)
			case markerITRK, markerITRKBug:
				d.Metadata.TrackNbr = nullTermStr(scratch)
			case markerITCH:
				d.Metadata.Technician = nullTermStr(scratch)
			case markerIKEY:
				d.Metadata.Keywords = nullTermStr(scratch)
			case markerIMED:
				d.Metadata.Medium = nullTermStr(scratch)
			}
		}
	}

	ch.Drain()

	return nil
}

func encodeInfoChunk(e *Encoder) []byte {
	if e == nil || e.Metadata == nil {
		return nil
	}

	buf := bytes.NewBuffer(nil)

	writeSection := func(id [4]byte, val string) {
		if val == "" {
			return
		}

		buf.Write(id[:])
		binary.Write(buf, binary.LittleEndian, uint32(len(val)+1))
		buf.Write(append([]byte(val), 0x00))
	}

	// Table-driven approach to reduce cyclomatic complexity
	fields := []struct {
		marker [4]byte
		value  string
	}{
		{markerIART, e.Metadata.Artist},
		{markerICMT, e.Metadata.Comments},
		{markerICOP, e.Metadata.Copyright},
		{markerICRD, e.Metadata.CreationDate},
		{markerIENG, e.Metadata.Engineer},
		{markerITCH, e.Metadata.Technician},
		{markerIGNR, e.Metadata.Genre},
		{markerIKEY, e.Metadata.Keywords},
		{markerIMED, e.Metadata.Medium},
		{markerINAM, e.Metadata.Title},
		{markerIPRD, e.Metadata.Product},
		{markerISBJ, e.Metadata.Subject},
		{markerISFT, e.Metadata.Software},
		{markerISRC, e.Metadata.Source},
		{markerIARL, e.Metadata.Location},
		{markerITRK, e.Metadata.TrackNbr},
	}

	for _, field := range fields {
		writeSection(field.marker, field.value)
	}

	return append(CIDInfo, buf.Bytes()...)
}
