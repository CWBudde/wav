package wav

import "testing"

func TestDecoderChunkAPIs(t *testing.T) {
	subFormat := makeSubFormatGUID(wavFormatPCM)
	d := &Decoder{
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

	gotFmt := d.FormatChunk()
	if gotFmt == nil || gotFmt.Extensible == nil {
		t.Fatal("expected fmt chunk copy")
	}

	if gotFmt == d.FmtChunk {
		t.Fatal("format chunk should be copied")
	}

	gotFmt.Extensible.ChannelMask = 0x4
	if d.FmtChunk.Extensible.ChannelMask != 0x3 {
		t.Fatal("format chunk copy should not mutate decoder")
	}

	raw := d.RawChunks()
	if len(raw) != 1 {
		t.Fatalf("expected 1 raw chunk, got %d", len(raw))
	}

	raw[0].Data[0] = 9
	if d.UnknownChunks[0].Data[0] != 1 {
		t.Fatal("raw chunks should be copied")
	}

	d.SetRawChunks([]RawChunk{{ID: [4]byte{'x', 't', 'r', 'a'}, Data: []byte{7, 8}}})

	if len(d.UnknownChunks) != 1 || d.UnknownChunks[0].ID != [4]byte{'x', 't', 'r', 'a'} {
		t.Fatalf("set raw chunks failed: %+v", d.UnknownChunks)
	}

	in := []RawChunk{{ID: [4]byte{'t', 'e', 's', 't'}, Data: []byte{4, 5, 6}}}
	d.SetRawChunks(in)

	in[0].Data[0] = 0
	if d.UnknownChunks[0].Data[0] != 4 {
		t.Fatal("SetRawChunks should copy input")
	}
}

func TestEncoderChunkAPIs(t *testing.T) {
	subFormat := makeSubFormatGUID(wavFormatPCM)
	e := &Encoder{
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

	gotFmt := e.FormatChunk()
	if gotFmt == nil || gotFmt.Extensible == nil {
		t.Fatal("expected fmt chunk copy")
	}

	if gotFmt == e.FmtChunk {
		t.Fatal("format chunk should be copied")
	}

	gotFmt.Extensible.ChannelMask = 0x4
	if e.FmtChunk.Extensible.ChannelMask != 0x3 {
		t.Fatal("format chunk copy should not mutate encoder")
	}

	raw := e.RawChunks()
	if len(raw) != 1 {
		t.Fatalf("expected 1 raw chunk, got %d", len(raw))
	}

	raw[0].Data[0] = 9
	if e.UnknownChunks[0].Data[0] != 1 {
		t.Fatal("raw chunks should be copied")
	}

	e.SetRawChunks([]RawChunk{{ID: [4]byte{'x', 't', 'r', 'a'}, Data: []byte{7, 8}}})

	if len(e.UnknownChunks) != 1 || e.UnknownChunks[0].ID != [4]byte{'x', 't', 'r', 'a'} {
		t.Fatalf("set raw chunks failed: %+v", e.UnknownChunks)
	}

	in := []RawChunk{{ID: [4]byte{'t', 'e', 's', 't'}, Data: []byte{4, 5, 6}}}
	e.SetRawChunks(in)

	in[0].Data[0] = 0
	if e.UnknownChunks[0].Data[0] != 4 {
		t.Fatal("SetRawChunks should copy input")
	}
}

func TestChunkAPIs_NilReceivers(t *testing.T) {
	var d *Decoder
	if d.FormatChunk() != nil {
		t.Fatal("nil decoder FormatChunk should be nil")
	}

	if d.RawChunks() != nil {
		t.Fatal("nil decoder RawChunks should be nil")
	}

	d.SetRawChunks([]RawChunk{{ID: [4]byte{'a', 'b', 'c', 'd'}}})

	var e *Encoder
	if e.FormatChunk() != nil {
		t.Fatal("nil encoder FormatChunk should be nil")
	}

	if e.RawChunks() != nil {
		t.Fatal("nil encoder RawChunks should be nil")
	}

	e.SetRawChunks([]RawChunk{{ID: [4]byte{'a', 'b', 'c', 'd'}}})
}
