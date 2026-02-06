package wav

import "testing"

func TestSearchSegment(t *testing.T) {
	tests := []struct {
		name  string
		value int
		table [8]int
		want  int
	}{
		{"first segment", 0x10, muLawSegmentEnd, 0},
		{"last segment", 0x1FFF, muLawSegmentEnd, 7},
		{"beyond all", 0x3FFF, muLawSegmentEnd, 8},
		{"alaw first", 0x10, aLawSegmentEnd, 0},
		{"alaw beyond", 0x2000, aLawSegmentEnd, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := searchSegment(tt.value, tt.table)
			if got != tt.want {
				t.Fatalf("searchSegment(%d)=%d, want %d", tt.value, got, tt.want)
			}
		})
	}
}

func TestMuLawRoundTrip(t *testing.T) {
	values := []int16{0, 100, -100, 1000, -1000, 8000, -8000, 32000, -32000}

	for _, v := range values {
		encoded := encodeMuLawSample(v)
		decoded := decodeMuLawSample(encoded)

		// mu-law is lossy, but round-trip should be close
		diff := int(v) - int(decoded)
		if diff < 0 {
			diff = -diff
		}

		// Allow larger tolerance for larger values
		tolerance := int(v) / 8
		if tolerance < 0 {
			tolerance = -tolerance
		}

		if tolerance < 100 {
			tolerance = 100
		}

		if diff > tolerance {
			t.Fatalf("mu-law round-trip failed for %d: encoded=%d, decoded=%d, diff=%d", v, encoded, decoded, diff)
		}
	}
}

func TestALawRoundTrip(t *testing.T) {
	values := []int16{0, 100, -100, 1000, -1000, 8000, -8000, 32000, -32000}

	for _, v := range values {
		encoded := encodeALawSample(v)
		decoded := decodeALawSample(encoded)

		diff := int(v) - int(decoded)
		if diff < 0 {
			diff = -diff
		}

		tolerance := int(v) / 8
		if tolerance < 0 {
			tolerance = -tolerance
		}

		if tolerance < 100 {
			tolerance = 100
		}

		if diff > tolerance {
			t.Fatalf("A-law round-trip failed for %d: encoded=%d, decoded=%d, diff=%d", v, encoded, decoded, diff)
		}
	}
}

func TestEncodeMuLawClip(t *testing.T) {
	// Values beyond clip range should be clamped
	high := encodeMuLawSample(32767)
	clip := encodeMuLawSample(int16(muLawClip * 4)) // Above clip

	// Both should produce valid encoded bytes
	if high == 0 && clip == 0 {
		t.Fatal("both encoded values should not be zero")
	}
}

func TestEncodeALawClip(t *testing.T) {
	high := encodeALawSample(32767)
	low := encodeALawSample(-32768)

	// Both should produce valid encoded bytes
	if high == low {
		t.Fatal("max and min should produce different encoded values")
	}
}
