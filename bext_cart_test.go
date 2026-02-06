package wav

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/go-audio/audio"
)

func TestDecoder_ReadMetadata_BWFBroadcastChunk(t *testing.T) {
	file, err := os.Open("fixtures/bwf.wav")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer file.Close()

	dec := NewDecoder(file)
	dec.ReadMetadata()

	if err := dec.Err(); err != nil {
		t.Fatalf("read metadata: %v", err)
	}

	if dec.Metadata == nil || dec.Metadata.BroadcastExtension == nil {
		t.Fatal("expected bext metadata from fixtures/bwf.wav")
	}

	for _, ch := range dec.UnknownChunks {
		if ch.ID == CIDBext {
			t.Fatal("bext chunk should be parsed as typed metadata, not unknown")
		}
	}
}

func TestBroadcastAndCartMetadataRoundTrip(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "bext_cart_roundtrip.wav")

	out, err := os.Create(outPath)
	if err != nil {
		t.Fatalf("create output: %v", err)
	}

	bextReserved := make([]byte, bextReservedLen)
	copy(bextReserved, []byte{0xaa, 0xbb, 0xcc})
	cartReserved := make([]byte, cartReservedLen)
	copy(cartReserved, []byte{0x10, 0x20, 0x30})

	var umid [64]byte
	copy(umid[:], []byte("UMID-0123456789"))

	post := [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}

	expectedBext := &BroadcastExtension{
		Description:         "BWF description",
		Originator:          "originator",
		OriginatorReference: "ref-001",
		OriginationDate:     "2026-02-06",
		OriginationTime:     "10:11:12",
		TimeReference:       1234567,
		Version:             1,
		UMID:                umid,
		Reserved:            bextReserved,
		CodingHistory:       "A=PCM,F=48000,W=16,M=mono,T=wav",
	}
	expectedCart := &Cart{
		Version:            "0101",
		Title:              "cart title",
		Artist:             "cart artist",
		CutID:              "CUT-42",
		ClientID:           "CLIENT-9",
		Category:           "PROMO",
		Classification:     "CL-1",
		OutCue:             "fade out",
		StartDate:          "2026-02-06",
		StartTime:          "10:00:00",
		EndDate:            "2026-02-07",
		EndTime:            "10:00:00",
		ProducerAppID:      "wav-tests",
		ProducerAppVersion: "1.0",
		UserDef:            "user",
		LevelReference:     123,
		PostTimer:          post,
		Reserved:           cartReserved,
		URL:                "https://example.test/item",
		TagText:            "tag payload",
	}

	enc := NewEncoder(out, 48000, 16, 1, wavFormatPCM)
	enc.Metadata = &Metadata{
		BroadcastExtension: expectedBext,
		Cart:               expectedCart,
	}
	buf := &audio.Float32Buffer{
		Format: &audio.Format{NumChannels: 1, SampleRate: 48000},
		Data:   []float32{0, 0.25, -0.25},
	}

	if err := enc.Write(buf); err != nil {
		t.Fatalf("encode data: %v", err)
	}

	if err := enc.Close(); err != nil {
		t.Fatalf("close encoder: %v", err)
	}

	if err := out.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	chunks, err := parseWavChunks(data)
	if err != nil {
		t.Fatalf("parse chunks: %v", err)
	}

	if ch, _ := findChunk(chunks, "bext"); ch == nil {
		t.Fatal("missing bext chunk in encoded file")
	}

	if ch, _ := findChunk(chunks, "cart"); ch == nil {
		t.Fatal("missing cart chunk in encoded file")
	}

	in, err := os.Open(outPath)
	if err != nil {
		t.Fatalf("open roundtrip: %v", err)
	}
	defer in.Close()

	dec := NewDecoder(in)
	dec.ReadMetadata()

	if err := dec.Err(); err != nil {
		t.Fatalf("read metadata: %v", err)
	}

	if dec.Metadata == nil {
		t.Fatal("metadata is nil")
	}

	if dec.Metadata.BroadcastExtension == nil {
		t.Fatal("broadcast extension metadata is nil")
	}

	if dec.Metadata.Cart == nil {
		t.Fatal("cart metadata is nil")
	}

	if !reflect.DeepEqual(dec.Metadata.BroadcastExtension, expectedBext) {
		t.Fatalf("bext mismatch:\n got: %#v\nwant: %#v", dec.Metadata.BroadcastExtension, expectedBext)
	}

	if !reflect.DeepEqual(dec.Metadata.Cart, expectedCart) {
		t.Fatalf("cart mismatch:\n got: %#v\nwant: %#v", dec.Metadata.Cart, expectedCart)
	}
}
