package wav

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	"github.com/go-audio/riff"
)

type testCustomListHandler struct {
	called bool
}

func (h *testCustomListHandler) CanHandle(chunkID [4]byte, listType [4]byte) bool {
	return chunkID == CIDList && bytes.Equal(listType[:], CIDInfo)
}

func (h *testCustomListHandler) Decode(_ *Decoder, ch *riff.Chunk) error {
	h.called = true

	_, err := io.ReadAll(ch.R)

	return err
}

func (h *testCustomListHandler) Encode(_ *Encoder) error {
	return nil
}

func TestChunkRegistryFactDecode(t *testing.T) {
	sampleCount := uint32(1234)
	payload := make([]byte, 4)
	binary.LittleEndian.PutUint32(payload, sampleCount)

	d := NewDecoder(bytes.NewReader(nil))
	ch := &riff.Chunk{ID: CIDFact, Size: 4, R: bytes.NewReader(payload)}

	handled, err := d.decodeChunkViaRegistry(ch)
	if err != nil {
		t.Fatalf("decode chunk via registry: %v", err)
	}

	if !handled {
		t.Fatal("expected fact chunk to be handled")
	}

	if d.CompressedSamples != sampleCount {
		t.Fatalf("compressed samples mismatch: got %d want %d", d.CompressedSamples, sampleCount)
	}
}

func TestChunkRegistrySupportsCustomListHandler(t *testing.T) {
	h := &testCustomListHandler{}
	registry := &ChunkRegistry{}
	registry.Register(h)

	d := NewDecoder(bytes.NewReader(nil))
	d.chunks = registry

	ch := &riff.Chunk{ID: CIDList, Size: 4, R: bytes.NewReader(CIDInfo)}

	handled, err := d.decodeChunkViaRegistry(ch)
	if err != nil {
		t.Fatalf("decode chunk via registry: %v", err)
	}

	if !handled {
		t.Fatal("expected custom LIST handler to be selected")
	}

	if !h.called {
		t.Fatal("expected custom LIST handler to be called")
	}
}

func TestChunkRegistryUnknownChunkFallback(t *testing.T) {
	dec := NewDecoder(bytes.NewReader(nil))
	dec.unknownChunkOrder = 1

	chunk := &riff.Chunk{ID: [4]byte{'t', 'e', 's', 't'}, Size: 3, R: bytes.NewReader([]byte{1, 2, 3})}

	handled, err := dec.decodeChunkViaRegistry(chunk)
	if err != nil {
		t.Fatalf("decode chunk via registry: %v", err)
	}

	if handled {
		t.Fatal("expected unknown chunk to be unhandled")
	}

	dec.captureUnknownChunk(chunk, true)

	if dec.Err() != nil {
		t.Fatalf("capture unknown chunk: %v", dec.Err())
	}

	if len(dec.UnknownChunks) != 1 {
		t.Fatalf("expected 1 unknown chunk, got %d", len(dec.UnknownChunks))
	}

	if dec.UnknownChunks[0].ID != [4]byte{'t', 'e', 's', 't'} {
		t.Fatalf("unknown id mismatch: %q", dec.UnknownChunks[0].ID)
	}

	if !bytes.Equal(dec.UnknownChunks[0].Data, []byte{1, 2, 3}) {
		t.Fatalf("unknown data mismatch: %v", dec.UnknownChunks[0].Data)
	}
}
