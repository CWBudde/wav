package wav

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	"github.com/go-audio/audio"
)

const regressionFullPCM = "FullPCMBuffer"

func TestDecoderIEEEFloatPreservesSamples(t *testing.T) {
	for _, depth := range []int{32, 64} {
		name := map[int]string{32: "float32", 64: "float64"}[depth]

		fixture, want := regressionFloatFixture(t, depth)
		for _, method := range []string{regressionFullPCM, "PCMBuffer"} {
			t.Run(name+"/"+method, func(t *testing.T) {
				dec := NewDecoder(bytes.NewReader(fixture))

				got := regressionDecodeSamples(t, dec, method)
				if len(got) != len(want) {
					t.Fatalf("decoded %d samples, want %d", len(got), len(want))
				}

				for i, sample := range got {
					if bits := math.Float32bits(sample); bits != want[i] {
						t.Fatalf("sample %d bits = %08x, want %08x", i, bits, want[i])
					}
				}
			})
		}
	}
}

func regressionFloatFixture(t *testing.T, depth int) ([]byte, []uint32) {
	t.Helper()
	// Serialize a standalone fixture rather than trusting the encoder to
	// construct what the decoder regression is meant to verify.
	var (
		payload bytes.Buffer
		want    []uint32
	)
	if depth == 32 {
		want = []uint32{
			0, 0x80000000, 0x40000000, 0xc0600000, 0x7f800000, 0xff800000,
			0x7fc12345, 0xffc54321, 0x7f812345, 1, 0x807fffff, 0x7f7fffff,
		}
		regressionWriteBinary(t, &payload, want)
	} else {
		bits64 := []uint64{
			0, 0x8000000000000000, 0x4000000000000000, 0xc00c000000000000,
			0x7ff0000000000000, 0xfff0000000000000, 0x7ff8123456789abc, 0xfff8abcdef123456,
			0x7ff0123456789abc, 1, 0x800fffffffffffff, 0x7fefffffffffffff,
		}
		regressionWriteBinary(t, &payload, bits64)

		for _, bits := range bits64 {
			want = append(want, math.Float32bits(float32(math.Float64frombits(bits))))
		}
	}

	var fixture bytes.Buffer
	fixture.WriteString("RIFF")
	regressionWriteBinary(t, &fixture, uint32(36+payload.Len()))
	fixture.WriteString("WAVEfmt ")

	for _, field := range []any{
		uint32(16), uint16(wavFormatIEEEFloat), uint16(1), uint32(48000),
		uint32(48000 * depth / 8), uint16(depth / 8), uint16(depth),
	} {
		regressionWriteBinary(t, &fixture, field)
	}

	fixture.WriteString("data")
	regressionWriteBinary(t, &fixture, uint32(payload.Len()))
	fixture.Write(payload.Bytes())

	return fixture.Bytes(), want
}

func regressionWriteBinary(t *testing.T, dst *bytes.Buffer, value any) {
	t.Helper()

	if err := binary.Write(dst, binary.LittleEndian, value); err != nil {
		t.Fatal(err)
	}
}

func regressionDecodeSamples(t *testing.T, dec *Decoder, method string) []float32 {
	t.Helper()

	if method == regressionFullPCM {
		buf, err := dec.FullPCMBuffer()
		if err != nil {
			t.Fatal(err)
		}

		return buf.Data
	}

	var got []float32

	buf := &audio.Float32Buffer{Data: make([]float32, 2)}
	for {
		count, err := dec.PCMBuffer(buf)
		if err != nil {
			t.Fatal(err)
		}

		if count == 0 {
			return got
		}

		got = append(got, buf.Data[:count]...)
	}
}

func TestDecoderOddDataExcludesPadding(t *testing.T) {
	data := encodeRegressionWAV(t, 8, wavFormatPCM, func(enc *Encoder) {
		enc.UnknownChunks = []RawChunk{{ID: [4]byte{'J', 'U', 'N', 'K'}, Data: []byte{3, 4, 5}}}

		enc.Metadata = &Metadata{Title: regressionOddTitle}
		if err := enc.Write(&audio.Float32Buffer{
			Format: &audio.Format{SampleRate: 48000, NumChannels: 1}, Data: []float32{-1, 0, 1},
		}); err != nil {
			t.Fatal(err)
		}
	})
	for _, method := range []string{regressionFullPCM, "PCMBuffer"} {
		t.Run(method, func(t *testing.T) {
			dec := NewDecoder(bytes.NewReader(data))

			got := regressionDecodeSamples(t, dec, method)
			if len(got) != 3 || dec.PCMSize != 3 {
				t.Fatalf("audio includes alignment padding: samples=%d bytes=%d", len(got), dec.PCMSize)
			}

			regressionMetadataAndRewind(t, dec)
		})
	}
}

func regressionMetadataAndRewind(t *testing.T, dec *Decoder) {
	t.Helper()
	dec.ReadMetadata()

	if err := dec.Err(); err != nil {
		t.Fatal(err)
	}

	if dec.Metadata == nil || dec.Metadata.Title != regressionOddTitle {
		t.Fatalf("post-data metadata was not decoded: %+v", dec.Metadata)
	}

	if err := dec.Rewind(); err != nil {
		t.Fatal(err)
	}

	buf, err := dec.FullPCMBuffer()
	if err != nil || len(buf.Data) != 3 {
		t.Fatalf("rewound decode: buffer=%+v err=%v", buf, err)
	}
}
