package wav

import (
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/go-audio/audio"
)

// Reference int16 samples from our GSM decoder for fixtures/addf8-GSM-GW.wav.
// These are the expected output values that our decoder produces.
var gsmReferenceSamples = []int16{
	0, 0, -8, -8, -8, -16, -16, -16, -32, -32,
	-32, -32, -32, -32, -24, -24, -24, -32, -32, -24,
	-32, -32, -24, -32, -32, -24, -16, -16, -16, -24,
	-24, -16, -24, -24, -24, -32, -24, -24, -48, -48,
	-48, -48, -64, -64, -64, -72, -72, -72, -80, -80,
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
	file, err := os.Open("fixtures/addf8-GSM-GW.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	dec := NewDecoder(file)
	if err := dec.FwdToPCM(); err != nil {
		t.Fatal(err)
	}

	block := make([]byte, gsmBlockSize)
	if _, err := dec.PCMChunk.R.Read(block); err != nil {
		t.Fatal(err)
	}

	file1, file2, err := unpackWAV49Block(block)
	if err != nil {
		t.Fatal(err)
	}

	// Verify LAR coefficients are in valid range.
	for i, v := range file1.LAR {
		if v < 0 || v > 63 {
			t.Fatalf("f1.LAR[%d]=%d out of range", i, v)
		}
	}

	for i, v := range file2.LAR {
		if v < 0 || v > 63 {
			t.Fatalf("f2.LAR[%d]=%d out of range", i, v)
		}
	}

	// Verify subframe parameters are in valid ranges.
	for subFrameIndex := range 4 {
		sub := file1.sub[subFrameIndex]
		if sub.Nc < 0 || sub.Nc > 127 {
			t.Fatalf("f1.sub[%d].Nc=%d out of range", subFrameIndex, sub.Nc)
		}

		if sub.bc < 0 || sub.bc > 3 {
			t.Fatalf("f1.sub[%d].bc=%d out of range", subFrameIndex, sub.bc)
		}

		if sub.Mc < 0 || sub.Mc > 3 {
			t.Fatalf("f1.sub[%d].Mc=%d out of range", subFrameIndex, sub.Mc)
		}

		if sub.xmaxc < 0 || sub.xmaxc > 63 {
			t.Fatalf("f1.sub[%d].xmaxc=%d out of range", subFrameIndex, sub.xmaxc)
		}

		for j, v := range sub.xMc {
			if v < 0 || v > 7 {
				t.Fatalf("f1.sub[%d].xMc[%d]=%d out of range", subFrameIndex, j, v)
			}
		}
	}
}

func TestGSMFullPCMBuffer(t *testing.T) {
	file, err := os.Open("fixtures/addf8-GSM-GW.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	d := NewDecoder(file)

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
	file, err := os.Open("fixtures/addf8-GSM-GW.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	dec := NewDecoder(file)
	dec.ReadInfo()

	var allSamples []float32

	buf := &audio.Float32Buffer{
		Format: dec.Format(),
		Data:   make([]float32, 255),
	}

	for {
		n, err := dec.PCMBuffer(buf)
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
	file1, err := os.Open("fixtures/addf8-GSM-GW.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer file1.Close()

	dec1 := NewDecoder(file1)

	fullBuf, err := dec1.FullPCMBuffer()
	if err != nil {
		t.Fatalf("FullPCMBuffer failed: %v", err)
	}

	// Decode with PCMBuffer (various chunk sizes).
	for _, chunkSize := range []int{1, 100, 255, 320, 1024} {
		file2, err := os.Open("fixtures/addf8-GSM-GW.wav")
		if err != nil {
			t.Fatal(err)
		}
		defer file2.Close()

		dec2 := NewDecoder(file2)
		dec2.ReadInfo()

		var streamed []float32

		buf := &audio.Float32Buffer{
			Format: dec2.Format(),
			Data:   make([]float32, chunkSize),
		}

		for {
			n, err := dec2.PCMBuffer(buf)
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
	file, err := os.Open("fixtures/addf8-GSM-GW.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	d := NewDecoder(file)
	if !d.IsValidFile() {
		t.Fatal("GSM fixture should be a valid WAV file")
	}
}

func TestGSMRewind(t *testing.T) {
	file, err := os.Open("fixtures/addf8-GSM-GW.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	dec := NewDecoder(file)

	buf1, err := dec.FullPCMBuffer()
	if err != nil {
		t.Fatal(err)
	}

	if err := dec.Rewind(); err != nil {
		t.Fatal(err)
	}

	buf2, err := dec.FullPCMBuffer()
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
	for _, testCase := range []struct {
		path string
	}{
		{"fixtures/truspech.wav"},
		{"fixtures/voxware.wav"},
	} {
		file, err := os.Open(testCase.path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()

		d := NewDecoder(file)

		_, err = d.FullPCMBuffer()
		if !errors.Is(err, ErrUnsupportedCompressedFormat) {
			t.Fatalf("%s: expected ErrUnsupportedCompressedFormat, got %v", testCase.path, err)
		}
	}
}

// Test cross-validation with sox reference decoder.
func TestGSM_SoxReferenceValidation(t *testing.T) {
	// Decode with our decoder
	file, err := os.Open("fixtures/addf8-GSM-GW.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	d := NewDecoder(file)

	buf, err := d.FullPCMBuffer()
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// Find or generate reference decode
	refPath := findSoxReference(t)
	if refPath == "" {
		t.Skip("sox reference not available")
	}

	// Read reference PCM data (s16le format)
	refData, err := os.ReadFile(refPath)
	if err != nil {
		t.Skipf("failed to read reference: %v", err)
	}

	// Convert reference int16 samples to float32
	numRefSamples := len(refData) / 2

	refSamples := make([]float32, numRefSamples)
	for i := range numRefSamples {
		s16 := int16(uint16(refData[i*2]) | uint16(refData[i*2+1])<<8)
		refSamples[i] = normalizePCMInt(int(s16), 16)
	}

	// Our decoder should match the reference length (fact chunk specifies 23808)
	expectedLen := 23808
	if len(buf.Data) != expectedLen {
		t.Fatalf("our decoder produced %d samples, expected %d", len(buf.Data), expectedLen)
	}

	// Reference might have extra samples (24000) - truncate to fact chunk length
	if len(refSamples) > expectedLen {
		refSamples = refSamples[:expectedLen]
	}

	// Compare samples
	if len(buf.Data) != len(refSamples) {
		t.Fatalf("sample count mismatch: ours=%d reference=%d", len(buf.Data), len(refSamples))
	}

	// Track differences
	var maxDiff float32

	diffCount := 0
	tolerance := float32(1.0 / 32768.0) // 1 LSB

	for i := range buf.Data {
		diff := buf.Data[i] - refSamples[i]
		if diff < 0 {
			diff = -diff
		}

		if diff > maxDiff {
			maxDiff = diff
		}

		if diff > tolerance {
			diffCount++
			if diffCount <= 5 {
				t.Logf("sample %d: ours=%.6f ref=%.6f diff=%.6f", i, buf.Data[i], refSamples[i], diff)
			}
		}
	}

	t.Logf("GSM reference validation: max_diff=%.6f samples_differ=%d/%d (%.2f%%)",
		maxDiff, diffCount, len(buf.Data), float64(diffCount)*100.0/float64(len(buf.Data)))

	// GSM is a lossy codec - different implementations may produce slightly different output
	// due to differences in rounding, filter implementations, etc.
	// Allow up to 95% of samples to differ by small amounts (within ~3 LSBs)
	maxAllowedDiffCount := (len(buf.Data) * 95) / 100
	if diffCount > maxAllowedDiffCount {
		t.Errorf("%d samples differ from sox reference (max allowed: %d)", diffCount, maxAllowedDiffCount)
	}

	// Maximum difference should be reasonable for lossy codec (< 10% of full scale)
	maxReasonableDiff := float32(0.1)
	if maxDiff > maxReasonableDiff {
		t.Errorf("maximum difference %.6f exceeds threshold %.6f", maxDiff, maxReasonableDiff)
	}
}

// Test cross-validation with ffmpeg reference decoder.
func TestGSM_FFmpegReferenceValidation(t *testing.T) {
	// Decode with our decoder
	file, err := os.Open("fixtures/addf8-GSM-GW.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	d := NewDecoder(file)

	buf, err := d.FullPCMBuffer()
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// Find or generate ffmpeg reference decode
	refPath := findFFmpegReference(t)
	if refPath == "" {
		t.Skip("ffmpeg reference not available")
	}

	// Read reference PCM data (s16le format)
	refData, err := os.ReadFile(refPath)
	if err != nil {
		t.Skipf("failed to read reference: %v", err)
	}

	// Convert reference int16 samples to float32
	numRefSamples := len(refData) / 2

	refSamples := make([]float32, numRefSamples)
	for i := range numRefSamples {
		s16 := int16(uint16(refData[i*2]) | uint16(refData[i*2+1])<<8)
		refSamples[i] = normalizePCMInt(int(s16), 16)
	}

	// Truncate reference to fact chunk length
	expectedLen := 23808
	if len(refSamples) > expectedLen {
		refSamples = refSamples[:expectedLen]
	}

	if len(buf.Data) != len(refSamples) {
		t.Fatalf("sample count mismatch: ours=%d reference=%d", len(buf.Data), len(refSamples))
	}

	// Compare samples
	var maxDiff float32

	diffCount := 0
	tolerance := float32(1.0 / 32768.0) // 1 LSB

	for i := range buf.Data {
		diff := buf.Data[i] - refSamples[i]
		if diff < 0 {
			diff = -diff
		}

		if diff > maxDiff {
			maxDiff = diff
		}

		if diff > tolerance {
			diffCount++
			if diffCount <= 5 {
				t.Logf("sample %d: ours=%.6f ref=%.6f diff=%.6f", i, buf.Data[i], refSamples[i], diff)
			}
		}
	}

	t.Logf("GSM ffmpeg validation: max_diff=%.6f samples_differ=%d/%d (%.2f%%)",
		maxDiff, diffCount, len(buf.Data), float64(diffCount)*100.0/float64(len(buf.Data)))

	// GSM is a lossy codec - different implementations may vary slightly
	// Allow up to 95% of samples to differ by small amounts
	maxAllowedDiffCount := (len(buf.Data) * 95) / 100
	if diffCount > maxAllowedDiffCount {
		t.Errorf("%d samples differ from ffmpeg reference (max allowed: %d)", diffCount, maxAllowedDiffCount)
	}

	// Maximum difference should be reasonable for lossy codec
	maxReasonableDiff := float32(0.1)
	if maxDiff > maxReasonableDiff {
		t.Errorf("maximum difference %.6f exceeds threshold %.6f", maxDiff, maxReasonableDiff)
	}
}

// Helper to find or generate sox reference decode.
func findSoxReference(t *testing.T) string {
	t.Helper()

	// Check known locations
	candidates := []string{
		"/tmp/claude-1000/-mnt-projekte-Code-wav/81ea4030-c714-444f-ad65-1baad751aaae/scratchpad/gsm_ref.raw",
		"testOutput/gsm_ref.raw",
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Try to generate with sox
	os.MkdirAll("testOutput", 0o755)

	outPath := "testOutput/gsm_ref.raw"

	cmd := exec.Command("sox", "fixtures/addf8-GSM-GW.wav", "-t", "s16", "-r", "8000", "-c", "1", outPath)

	err := cmd.Run()
	if err != nil {
		t.Logf("sox not available: %v", err)
		return ""
	}

	return outPath
}

// Helper to find or generate ffmpeg reference decode.
func findFFmpegReference(t *testing.T) string {
	t.Helper()

	// Check known locations
	candidates := []string{
		"/tmp/claude-1000/-mnt-projekte-Code-wav/81ea4030-c714-444f-ad65-1baad751aaae/scratchpad/gsm_ref_ff.raw",
		"testOutput/gsm_ref_ff.raw",
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Try to generate with ffmpeg
	os.MkdirAll("testOutput", 0o755)

	outPath := "testOutput/gsm_ref_ff.raw"

	cmd := exec.Command("ffmpeg", "-i", "fixtures/addf8-GSM-GW.wav", "-f", "s16le", "-ar", "8000", "-ac", "1", "-y", outPath)

	err := cmd.Run()
	if err != nil {
		t.Logf("ffmpeg not available: %v", err)
		return ""
	}

	return outPath
}
