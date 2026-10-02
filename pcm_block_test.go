package wav

import (
	"bytes"
	"fmt"
	"math"
	"testing"

	"github.com/go-audio/audio"
	"github.com/go-audio/riff"
)

func TestPCMBlockMatchesSampleDecoder(t *testing.T) {
	tests := []struct {
		format uint16
		bits   int
	}{
		{wavFormatPCM, 8},
		{wavFormatPCM, 12},
		{wavFormatPCM, 16},
		{wavFormatPCM, 20},
		{wavFormatPCM, 24},
		{wavFormatPCM, 28},
		{wavFormatPCM, 32},
		{wavFormatIEEEFloat, 32},
		{wavFormatIEEEFloat, 64},
		{wavFormatALaw, 8},
		{wavFormatMuLaw, 8},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("format%d/bits%d", test.format, test.bits), func(t *testing.T) {
			runPCMBlockParity(t, test.format, test.bits)
		})
	}
}

func runPCMBlockParity(t *testing.T, format uint16, bits int) {
	t.Helper()

	const count = 4096

	width := bytesPerSample(bits)

	payload := make([]byte, count*width)
	for i := range payload {
		payload[i] = byte(i*73 + i/7 + 19)
	}

	decodeBlock, err := sampleDecodeBlockFunc(bits, format)
	if err != nil {
		t.Fatal(err)
	}

	decodeSample, err := sampleDecodeFloat32Func(bits, format)
	if err != nil {
		t.Fatal(err)
	}

	got := make([]float32, count)
	decodeBlock(got, payload)
	reader := bytes.NewReader(payload)

	scratch := make([]byte, width)
	for i, sample := range got {
		want, err := decodeSample(reader, scratch)
		if err != nil {
			t.Fatal(err)
		}

		if math.Float32bits(sample) != math.Float32bits(want) {
			t.Fatalf("sample %d bits = %08x, want %08x", i, math.Float32bits(sample), math.Float32bits(want))
		}
	}
}

// Limit individual reads to one byte, including reads in the middle of a sample.
type shortPCMReader struct {
	*bytes.Reader
}

func (r shortPCMReader) Read(dst []byte) (int, error) {
	return r.Reader.Read(dst[:min(len(dst), 1)]) //nolint:wrapcheck // io.ReadFull requires the exact io.EOF sentinel.
}

func TestPCMBufferAssemblesShortReads(t *testing.T) {
	fixture, want := regressionFloatFixture(t, 32)
	dec := NewDecoder(shortPCMReader{bytes.NewReader(fixture)})

	got := regressionDecodeSamples(t, dec, "PCMBuffer")
	if len(got) != len(want) {
		t.Fatalf("decoded %d samples, want %d", len(got), len(want))
	}

	for i, sample := range got {
		if math.Float32bits(sample) != want[i] {
			t.Fatalf("sample %d bits = %08x, want %08x", i, math.Float32bits(sample), want[i])
		}
	}
}

func TestPCMBufferReusesScratch(t *testing.T) {
	payload := make([]byte, 4096)
	reader := bytes.NewReader(payload)
	dec := &Decoder{
		NumChans: 1, SampleRate: 48000, BitDepth: 16, WavAudioFormat: wavFormatPCM,
		pcmDataAccessed: true, PCMChunk: &riff.Chunk{R: reader},
	}

	buf := &audio.Float32Buffer{Data: make([]float32, len(payload)/2)}
	if _, err := dec.PCMBuffer(buf); err != nil {
		t.Fatal(err)
	}

	backing := &dec.pcmScratch[0]

	allocs := testing.AllocsPerRun(100, func() {
		reader.Reset(payload)

		if count, err := dec.PCMBuffer(buf); err != nil || count != len(buf.Data) {
			t.Fatalf("PCMBuffer count=%d err=%v", count, err)
		}
	})
	if allocs != 0 {
		t.Fatalf("warmed streaming block allocates %.0f times, want zero", allocs)
	}

	if &dec.pcmScratch[0] != backing {
		t.Fatal("streaming blocks replaced reusable scratch storage")
	}
}

func BenchmarkPCMBufferPCM16(b *testing.B) {
	const samples = 32768

	payload := make([]byte, samples*2)
	reader := bytes.NewReader(payload)
	dec := &Decoder{
		NumChans: 2, SampleRate: 48000, BitDepth: 16, WavAudioFormat: wavFormatPCM,
		pcmDataAccessed: true, PCMChunk: &riff.Chunk{R: reader},
	}

	buf := &audio.Float32Buffer{Data: make([]float32, samples)}
	if _, err := dec.PCMBuffer(buf); err != nil {
		b.Fatal(err)
	}

	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		reader.Reset(payload)

		if _, err := dec.PCMBuffer(buf); err != nil {
			b.Fatal(err)
		}
	}
}
