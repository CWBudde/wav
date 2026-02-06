package wav

import "testing"

func TestDecoderChunkAPIs(t *testing.T) {
	subFormat := makeSubFormatGUID(wavFormatPCM)
	dec := &Decoder{
		FmtChunk: &FmtChunk{
			FormatTag: wavFormatExtensible,
			Extensible: &FmtExtensible{
				ValidBitsPerSample: 16,
				ChannelMask:        0x3,
				SubFormat:          subFormat,
			},
		},
		UnknownChunks: []RawChunk{
			{ID: [4]byte{'J', 'U', 'N', 'K'}, Data: []byte{1, 2, 3}, BeforeData: true},
		},
	}

	gotFmt := dec.FormatChunk()
	if gotFmt == nil || gotFmt.Extensible == nil {
		t.Fatal("expected fmt chunk copy")
	}

	if gotFmt == dec.FmtChunk {
		t.Fatal("format chunk should be copied")
	}

	gotFmt.Extensible.ChannelMask = 0x4
	if dec.FmtChunk.Extensible.ChannelMask != 0x3 {
		t.Fatal("format chunk copy should not mutate decoder")
	}

	raw := dec.RawChunks()
	if len(raw) != 1 {
		t.Fatalf("expected 1 raw chunk, got %d", len(raw))
	}

	raw[0].Data[0] = 9
	if dec.UnknownChunks[0].Data[0] != 1 {
		t.Fatal("raw chunks should be copied")
	}

	dec.SetRawChunks([]RawChunk{{ID: [4]byte{'x', 't', 'r', 'a'}, Data: []byte{7, 8}}})

	if len(dec.UnknownChunks) != 1 || dec.UnknownChunks[0].ID != [4]byte{'x', 't', 'r', 'a'} {
		t.Fatalf("set raw chunks failed: %+v", dec.UnknownChunks)
	}

	in := []RawChunk{{ID: [4]byte{'t', 'e', 's', 't'}, Data: []byte{4, 5, 6}}}
	dec.SetRawChunks(in)

	in[0].Data[0] = 0
	if dec.UnknownChunks[0].Data[0] != 4 {
		t.Fatal("SetRawChunks should copy input")
	}
}

func TestEncoderChunkAPIs(t *testing.T) {
	subFormat := makeSubFormatGUID(wavFormatPCM)
	enc := &Encoder{
		FmtChunk: &FmtChunk{
			FormatTag: wavFormatExtensible,
			Extensible: &FmtExtensible{
				ValidBitsPerSample: 16,
				ChannelMask:        0x3,
				SubFormat:          subFormat,
			},
		},
		UnknownChunks: []RawChunk{
			{ID: [4]byte{'J', 'U', 'N', 'K'}, Data: []byte{1, 2, 3}, BeforeData: false},
		},
	}

	gotFmt := enc.FormatChunk()
	if gotFmt == nil || gotFmt.Extensible == nil {
		t.Fatal("expected fmt chunk copy")
	}

	if gotFmt == enc.FmtChunk {
		t.Fatal("format chunk should be copied")
	}

	gotFmt.Extensible.ChannelMask = 0x4
	if enc.FmtChunk.Extensible.ChannelMask != 0x3 {
		t.Fatal("format chunk copy should not mutate encoder")
	}

	raw := enc.RawChunks()
	if len(raw) != 1 {
		t.Fatalf("expected 1 raw chunk, got %d", len(raw))
	}

	raw[0].Data[0] = 9
	if enc.UnknownChunks[0].Data[0] != 1 {
		t.Fatal("raw chunks should be copied")
	}

	enc.SetRawChunks([]RawChunk{{ID: [4]byte{'x', 't', 'r', 'a'}, Data: []byte{7, 8}}})

	if len(enc.UnknownChunks) != 1 || enc.UnknownChunks[0].ID != [4]byte{'x', 't', 'r', 'a'} {
		t.Fatalf("set raw chunks failed: %+v", enc.UnknownChunks)
	}

	in := []RawChunk{{ID: [4]byte{'t', 'e', 's', 't'}, Data: []byte{4, 5, 6}}}
	enc.SetRawChunks(in)

	in[0].Data[0] = 0
	if enc.UnknownChunks[0].Data[0] != 4 {
		t.Fatal("SetRawChunks should copy input")
	}
}

func TestChunkAPIs_NilReceivers(t *testing.T) {
	var dec *Decoder
	if dec.FormatChunk() != nil {
		t.Fatal("nil decoder FormatChunk should be nil")
	}

	if dec.RawChunks() != nil {
		t.Fatal("nil decoder RawChunks should be nil")
	}

	dec.SetRawChunks([]RawChunk{{ID: [4]byte{'a', 'b', 'c', 'd'}}})

	var enc *Encoder
	if enc.FormatChunk() != nil {
		t.Fatal("nil encoder FormatChunk should be nil")
	}

	if enc.RawChunks() != nil {
		t.Fatal("nil encoder RawChunks should be nil")
	}

	enc.SetRawChunks([]RawChunk{{ID: [4]byte{'a', 'b', 'c', 'd'}}})
}
