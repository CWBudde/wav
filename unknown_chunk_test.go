package wav

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestUnknownChunkRoundTripPreservesPayloadAndOrder(t *testing.T) {
	input := makeWavWithUnknownChunks(t)

	dec := NewDecoder(bytes.NewReader(input))
	dec.ReadMetadata()

	if err := dec.Err(); err != nil {
		t.Fatalf("read metadata: %v", err)
	}

	if err := dec.Rewind(); err != nil {
		t.Fatalf("rewind: %v", err)
	}

	buf, err := dec.FullPCMBuffer()
	if err != nil {
		t.Fatalf("decode PCM: %v", err)
	}

	if len(dec.UnknownChunks) != 2 {
		t.Fatalf("expected 2 unknown chunks, got %d", len(dec.UnknownChunks))
	}

	if dec.UnknownChunks[0].ID != [4]byte{'J', 'U', 'N', 'K'} {
		t.Fatalf("first unknown chunk id mismatch: %q", dec.UnknownChunks[0].ID)
	}

	if !dec.UnknownChunks[0].BeforeData {
		t.Fatal("expected first unknown chunk to be before data")
	}

	if dec.UnknownChunks[1].ID != [4]byte{'x', 't', 'r', 'a'} {
		t.Fatalf("second unknown chunk id mismatch: %q", dec.UnknownChunks[1].ID)
	}

	if dec.UnknownChunks[1].BeforeData {
		t.Fatal("expected second unknown chunk to be after data")
	}

	outPath := filepath.Join(t.TempDir(), "unknown_roundtrip.wav")

	out, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}

	enc := NewEncoderFromDecoder(out, dec)

	if err := enc.Write(buf); err != nil {
		t.Fatalf("encode: %v", err)
	}

	if err := enc.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if err := out.Close(); err != nil {
		t.Fatalf("file close: %v", err)
	}

	output, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}

	chunks, err := parseWavChunks(output)
	if err != nil {
		t.Fatalf("parse output wav chunks: %v", err)
	}

	pre, prePos := findChunk(chunks, "JUNK")
	if pre == nil {
		t.Fatal("missing preserved JUNK chunk")
	}

	if !bytes.Equal(pre.data, []byte{0x01, 0x02, 0x03, 0x04}) {
		t.Fatalf("JUNK payload mismatch: got %v", pre.data)
	}

	dataChunk, dataPos := findChunk(chunks, "data")
	if dataChunk == nil {
		t.Fatal("missing data chunk")
	}

	post, postPos := findChunk(chunks, "xtra")
	if post == nil {
		t.Fatal("missing preserved xtra chunk")
	}

	if !bytes.Equal(post.data, []byte{0x09, 0x08, 0x07, 0x06}) {
		t.Fatalf("xtra payload mismatch: got %v", post.data)
	}

	if prePos >= dataPos || dataPos >= postPos {
		t.Fatalf("chunk order mismatch: JUNK=%d data=%d xtra=%d", prePos, dataPos, postPos)
	}
}

type testChunk struct {
	id   string
	size uint32
	data []byte
}

func makeWavWithUnknownChunks(t *testing.T) []byte {
	t.Helper()

	var b bytes.Buffer
	b.WriteString("RIFF")

	err := binary.Write(&b, binary.LittleEndian, uint32(0))
	if err != nil {
		t.Fatalf("write riff size placeholder: %v", err)
	}

	b.WriteString("WAVE")

	fmtPayload := make([]byte, 16)
	binary.LittleEndian.PutUint16(fmtPayload[0:2], wavFormatPCM)
	binary.LittleEndian.PutUint16(fmtPayload[2:4], 1)
	binary.LittleEndian.PutUint32(fmtPayload[4:8], 8000)
	binary.LittleEndian.PutUint32(fmtPayload[8:12], 16000)
	binary.LittleEndian.PutUint16(fmtPayload[12:14], 2)
	binary.LittleEndian.PutUint16(fmtPayload[14:16], 16)
	writeTestChunk(t, &b, "fmt ", fmtPayload)
	writeTestChunk(t, &b, "JUNK", []byte{0x01, 0x02, 0x03, 0x04})
	writeTestChunk(t, &b, "data", []byte{0x01, 0x00, 0x02, 0x00})
	writeTestChunk(t, &b, "xtra", []byte{0x09, 0x08, 0x07, 0x06})

	out := b.Bytes()
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(out)-8))

	return out
}

func writeTestChunk(t *testing.T, b *bytes.Buffer, id string, payload []byte) {
	t.Helper()

	if len(id) != 4 {
		t.Fatalf("chunk id must be 4 bytes, got %q", id)
	}

	b.WriteString(id)

	err := binary.Write(b, binary.LittleEndian, uint32(len(payload)))
	if err != nil {
		t.Fatalf("write chunk size for %q: %v", id, err)
	}

	if _, err := b.Write(payload); err != nil {
		t.Fatalf("write chunk payload for %q: %v", id, err)
	}

	if len(payload)%2 == 1 {
		err := b.WriteByte(0)
		if err != nil {
			t.Fatalf("write chunk pad for %q: %v", id, err)
		}
	}
}

func parseWavChunks(data []byte) ([]testChunk, error) {
	if len(data) < 12 {
		return nil, errors.New("file too small")
	}

	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, errors.New("invalid riff/wave header")
	}

	chunks := make([]testChunk, 0)

	offset := 12
	for offset+8 <= len(data) {
		id := string(data[offset : offset+4])
		size := binary.LittleEndian.Uint32(data[offset+4 : offset+8])
		offset += 8

		end := offset + int(size)
		if end > len(data) {
			return nil, fmt.Errorf("chunk %q exceeds file size", id)
		}

		payload := append([]byte(nil), data[offset:end]...)
		chunks = append(chunks, testChunk{id: id, size: size, data: payload})

		offset = end
		if size%2 == 1 {
			offset++
		}
	}

	return chunks, nil
}

func findChunk(chunks []testChunk, id string) (*testChunk, int) {
	for i := range chunks {
		if chunks[i].id == id {
			return &chunks[i], i
		}
	}

	return nil, -1
}
