package wav

import (
	"errors"
	"os"
	"testing"

	"github.com/go-audio/audio"
)

// Reference int16 samples from sox/ffmpeg decode of fixtures/addf8-GSM-GW.wav.
var gsmReferenceSamples = []int16{
	0, 0, -8, -8, -8, -16, -16, -16, -32, -32,
	-32, -32, -32, -32, -24, -24, -24, -24, -24, -24,
	-32, -24, -24, -32, -32, -24, -16, -8, -8, -16,
	-16, -16, -24, -24, -24, -32, -32, -32, -56, -56,
	-48, -48, -64, -64, -64, -72, -72, -64, -64, -64,
}

func TestGsmFixedPointArithmetic(t *testing.T) {
	t.Run("gsmAdd", func(t *testing.T) {
		if got := gsmAdd(32767, 1); got != 32767 {
			t.Fatalf("overflow: got %d, want 32767", got)
		}
		if got := gsmAdd(-32768, -1); got != -32768 {
			t.Fatalf("underflow: got %d, want -32768", got)
		}
		if got := gsmAdd(100, 200); got != 300 {
			t.Fatalf("normal: got %d, want 300", got)
		}
	})

	t.Run("gsmSub", func(t *testing.T) {
		if got := gsmSub(-32768, 1); got != -32768 {
			t.Fatalf("underflow: got %d, want -32768", got)
		}
		if got := gsmSub(32767, -1); got != 32767 {
			t.Fatalf("overflow: got %d, want 32767", got)
		}
		if got := gsmSub(300, 100); got != 200 {
			t.Fatalf("normal: got %d, want 200", got)
		}
	})

	t.Run("gsmMultR", func(t *testing.T) {
		if got := gsmMultR(-32768, -32768); got != 32767 {
			t.Fatalf("min*min: got %d, want 32767", got)
		}
		if got := gsmMultR(16384, 16384); got != 8192 {
			t.Fatalf("quarter*quarter: got %d, want 8192", got)
		}
	})

	t.Run("gsmAbs", func(t *testing.T) {
		if got := gsmAbs(-32768); got != 32767 {
			t.Fatalf("min: got %d, want 32767", got)
		}
		if got := gsmAbs(-100); got != 100 {
			t.Fatalf("neg: got %d, want 100", got)
		}
		if got := gsmAbs(42); got != 42 {
			t.Fatalf("pos: got %d, want 42", got)
		}
	})
}

func TestGsmUnpackBlock(t *testing.T) {
	f, err := os.Open("fixtures/addf8-GSM-GW.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	d := NewDecoder(f)
	if err := d.FwdToPCM(); err != nil {
		t.Fatal(err)
	}

	block := make([]byte, gsmBlockSize)
	if _, err := d.PCMChunk.R.Read(block); err != nil {
		t.Fatal(err)
	}

	f1, f2, err := unpackWAV49Block(block)
	if err != nil {
		t.Fatal(err)
	}

	// Verify LAR coefficients are in valid range.
	for i, v := range f1.LAR {
		if v < 0 || v > 63 {
			t.Fatalf("f1.LAR[%d]=%d out of range", i, v)
		}
	}
	for i, v := range f2.LAR {
		if v < 0 || v > 63 {
			t.Fatalf("f2.LAR[%d]=%d out of range", i, v)
		}
	}

	// Verify subframe parameters are in valid ranges.
	for s := 0; s < 4; s++ {
		sub := f1.sub[s]
		if sub.Nc < 0 || sub.Nc > 127 {
			t.Fatalf("f1.sub[%d].Nc=%d out of range", s, sub.Nc)
		}
		if sub.bc < 0 || sub.bc > 3 {
			t.Fatalf("f1.sub[%d].bc=%d out of range", s, sub.bc)
		}
		if sub.Mc < 0 || sub.Mc > 3 {
			t.Fatalf("f1.sub[%d].Mc=%d out of range", s, sub.Mc)
		}
		if sub.xmaxc < 0 || sub.xmaxc > 63 {
			t.Fatalf("f1.sub[%d].xmaxc=%d out of range", s, sub.xmaxc)
		}
		for j, v := range sub.xMc {
			if v < 0 || v > 7 {
				t.Fatalf("f1.sub[%d].xMc[%d]=%d out of range", s, j, v)
			}
		}
	}
}

func TestGSMFullPCMBuffer(t *testing.T) {
	f, err := os.Open("fixtures/addf8-GSM-GW.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	d := NewDecoder(f)
	buf, err := d.FullPCMBuffer()
	if err != nil {
		t.Fatalf("FullPCMBuffer failed: %v", err)
	}

	if buf.Format.SampleRate != 8000 {
		t.Fatalf("expected sample rate 8000, got %d", buf.Format.SampleRate)
	}
	if buf.Format.NumChannels != 1 {
		t.Fatalf("expected 1 channel, got %d", buf.Format.NumChannels)
	}
	if buf.SourceBitDepth != 16 {
		t.Fatalf("expected source bit depth 16, got %d", buf.SourceBitDepth)
	}

	// Fact chunk says 23808 samples.
	if len(buf.Data) != 23808 {
		t.Fatalf("expected 23808 samples, got %d", len(buf.Data))
	}

	// All samples should be in [-1, 1].
	for i, v := range buf.Data {
		if v < -1 || v > 1 {
			t.Fatalf("sample %d = %f out of range [-1, 1]", i, v)
		}
	}

	// Compare first 50 samples against reference.
	for i, ref := range gsmReferenceSamples {
		expected := normalizePCMInt(int(ref), 16)
		if !float32ApproxEqual(buf.Data[i], expected, 1e-5) {
			t.Fatalf("sample %d: got %f, want %f (ref int16=%d)", i, buf.Data[i], expected, ref)
		}
	}
}

func TestGSMPCMBuffer(t *testing.T) {
	f, err := os.Open("fixtures/addf8-GSM-GW.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	d := NewDecoder(f)
	d.ReadInfo()

	var allSamples []float32
	buf := &audio.Float32Buffer{
		Format: d.Format(),
		Data:   make([]float32, 255),
	}

	for {
		n, err := d.PCMBuffer(buf)
		if err != nil {
			t.Fatalf("PCMBuffer failed: %v", err)
		}
		if n == 0 {
			break
		}

		allSamples = append(allSamples, buf.Data[:n]...)
	}

	if len(allSamples) != 23808 {
		t.Fatalf("expected 23808 total samples, got %d", len(allSamples))
	}

	// Compare first 50 samples against reference.
	for i, ref := range gsmReferenceSamples {
		expected := normalizePCMInt(int(ref), 16)
		if !float32ApproxEqual(allSamples[i], expected, 1e-5) {
			t.Fatalf("sample %d: got %f, want %f", i, allSamples[i], expected)
		}
	}
}

func TestGSMFullPCMBuffer_PCMBuffer_Parity(t *testing.T) {
	// Decode with FullPCMBuffer.
	f1, err := os.Open("fixtures/addf8-GSM-GW.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer f1.Close()

	d1 := NewDecoder(f1)
	fullBuf, err := d1.FullPCMBuffer()
	if err != nil {
		t.Fatalf("FullPCMBuffer failed: %v", err)
	}

	// Decode with PCMBuffer (various chunk sizes).
	for _, chunkSize := range []int{1, 100, 255, 320, 1024} {
		f2, err := os.Open("fixtures/addf8-GSM-GW.wav")
		if err != nil {
			t.Fatal(err)
		}
		defer f2.Close()

		d2 := NewDecoder(f2)
		d2.ReadInfo()

		var streamed []float32
		buf := &audio.Float32Buffer{
			Format: d2.Format(),
			Data:   make([]float32, chunkSize),
		}

		for {
			n, err := d2.PCMBuffer(buf)
			if err != nil {
				t.Fatalf("chunk=%d: PCMBuffer failed: %v", chunkSize, err)
			}
			if n == 0 {
				break
			}

			streamed = append(streamed, buf.Data[:n]...)
		}

		if len(streamed) != len(fullBuf.Data) {
			t.Fatalf("chunk=%d: length mismatch: %d vs %d", chunkSize, len(streamed), len(fullBuf.Data))
		}

		for i := range streamed {
			if streamed[i] != fullBuf.Data[i] {
				t.Fatalf("chunk=%d: sample %d mismatch: %f vs %f", chunkSize, i, streamed[i], fullBuf.Data[i])
			}
		}
	}
}

func TestGSMDecodeBlock_ZeroBlock(t *testing.T) {
	dec := newGSMDecoder(0)
	block := make([]byte, gsmBlockSize)

	_, err := dec.decodeBlock(block)
	if err != nil {
		t.Fatalf("decoding zero block should not error: %v", err)
	}
}

func TestGSMDecodeBlock_InvalidSize(t *testing.T) {
	dec := newGSMDecoder(0)

	_, err := dec.decodeBlock(make([]byte, 32))
	if err == nil {
		t.Fatal("expected error for short block")
	}
}

func TestGSMIsValidFile(t *testing.T) {
	f, err := os.Open("fixtures/addf8-GSM-GW.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	d := NewDecoder(f)
	if !d.IsValidFile() {
		t.Fatal("GSM fixture should be a valid WAV file")
	}
}

func TestGSMRewind(t *testing.T) {
	f, err := os.Open("fixtures/addf8-GSM-GW.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	d := NewDecoder(f)
	buf1, err := d.FullPCMBuffer()
	if err != nil {
		t.Fatal(err)
	}

	if err := d.Rewind(); err != nil {
		t.Fatal(err)
	}

	buf2, err := d.FullPCMBuffer()
	if err != nil {
		t.Fatal(err)
	}

	if len(buf1.Data) != len(buf2.Data) {
		t.Fatalf("rewind: length mismatch %d vs %d", len(buf1.Data), len(buf2.Data))
	}

	for i := range buf1.Data {
		if buf1.Data[i] != buf2.Data[i] {
			t.Fatalf("rewind: sample %d mismatch", i)
		}
	}
}

func TestUnsupportedFormatsStillFail(t *testing.T) {
	// TrueSpeech and Voxware should still be unsupported.
	for _, tc := range []struct {
		path string
	}{
		{"fixtures/truspech.wav"},
		{"fixtures/voxware.wav"},
	} {
		f, err := os.Open(tc.path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		d := NewDecoder(f)
		_, err = d.FullPCMBuffer()
		if !errors.Is(err, ErrUnsupportedCompressedFormat) {
			t.Fatalf("%s: expected ErrUnsupportedCompressedFormat, got %v", tc.path, err)
		}
	}
}
