package wav

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-audio/audio"
)

func TestNewEncoderFromDecoder_CopiesRoundTripFields(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "from_decoder.wav")
	out, err := os.Create(outPath)
	if err != nil {
		t.Fatalf("create output: %v", err)
	}
	defer out.Close()

	subFormat := makeSubFormatGUID(wavFormatPCM)
	dec := &Decoder{
		SampleRate:     44100,
		BitDepth:       16,
		NumChans:       2,
		WavAudioFormat: wavFormatPCM,
		FmtChunk: &FmtChunk{
			FormatTag: wavFormatExtensible,
			Extensible: &FmtExtensible{
				ValidBitsPerSample: 16,
				ChannelMask:        0x3,
				SubFormat:          subFormat,
			},
		},
		UnknownChunks: []RawChunk{
			{ID: [4]byte{'J', 'U', 'N', 'K'}, Size: 3, Data: []byte{1, 2, 3}, BeforeData: true},
		},
	}

	enc := NewEncoderFromDecoder(out, dec)
	if enc.SampleRate != int(dec.SampleRate) {
		t.Fatalf("sample rate mismatch: got %d want %d", enc.SampleRate, dec.SampleRate)
	}
	if enc.BitDepth != int(dec.BitDepth) {
		t.Fatalf("bit depth mismatch: got %d want %d", enc.BitDepth, dec.BitDepth)
	}
	if enc.NumChans != int(dec.NumChans) {
		t.Fatalf("channels mismatch: got %d want %d", enc.NumChans, dec.NumChans)
	}
	if enc.WavAudioFormat != int(dec.WavAudioFormat) {
		t.Fatalf("audio format mismatch: got %d want %d", enc.WavAudioFormat, dec.WavAudioFormat)
	}

	if enc.FmtChunk == nil || enc.FmtChunk.Extensible == nil {
		t.Fatal("expected fmt chunk copy with extensible fields")
	}
	if enc.FmtChunk == dec.FmtChunk {
		t.Fatal("fmt chunk should be deep-copied")
	}

	if len(enc.UnknownChunks) != 1 {
		t.Fatalf("unknown chunk count mismatch: got %d", len(enc.UnknownChunks))
	}
	if &enc.UnknownChunks[0] == &dec.UnknownChunks[0] {
		t.Fatal("unknown chunk should be deep-copied")
	}
	if !strings.EqualFold(string(enc.UnknownChunks[0].ID[:]), "junk") {
		t.Fatalf("unexpected unknown chunk id: %q", enc.UnknownChunks[0].ID)
	}

	dec.UnknownChunks[0].Data[0] = 9
	if enc.UnknownChunks[0].Data[0] != 1 {
		t.Fatal("encoder unknown chunk data should not share backing storage with decoder")
	}
}

func TestEncoderWriteFrameFloat64(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "frame_float64.wav")
	out, err := os.Create(outPath)
	if err != nil {
		t.Fatalf("create output: %v", err)
	}

	enc := NewEncoder(out, 48000, 16, 1, wavFormatPCM)
	if err := enc.WriteFrame(float64(0.5)); err != nil {
		t.Fatalf("WriteFrame failed: %v", err)
	}

	if err := enc.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if err := out.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}

	in, err := os.Open(outPath)
	if err != nil {
		t.Fatalf("open encoded file: %v", err)
	}
	defer in.Close()

	dec := NewDecoder(in)
	buf, err := dec.FullPCMBuffer()
	if err != nil {
		t.Fatalf("decode PCM buffer: %v", err)
	}

	if len(buf.Data) != 1 {
		t.Fatalf("expected one sample, got %d", len(buf.Data))
	}

	if !float32ApproxEqual(buf.Data[0], 0.5, 1e-4) {
		t.Fatalf("decoded sample=%f, want ~0.5", buf.Data[0])
	}
}

func TestEncoderWriteFrameErrors(t *testing.T) {
	out, err := os.Create(filepath.Join(t.TempDir(), "err.wav"))
	if err != nil {
		t.Fatalf("create output: %v", err)
	}
	defer out.Close()

	t.Run("unsupported wav format", func(t *testing.T) {
		enc := NewEncoder(out, 44100, 16, 1, 999)
		err := enc.WriteFrame(float32(0))
		if err == nil || !strings.Contains(err.Error(), "unsupported wav format") {
			t.Fatalf("expected unsupported wav format error, got %v", err)
		}
	})

	t.Run("unsupported bit depth", func(t *testing.T) {
		enc := NewEncoder(out, 44100, 12, 1, wavFormatPCM)
		err := enc.WriteFrame(float32(0))
		if err == nil || !strings.Contains(err.Error(), "can't add frames of bit size") {
			t.Fatalf("expected unsupported bit depth error, got %v", err)
		}
	})
}

func TestEncoderWriteNilBuffer(t *testing.T) {
	out, err := os.Create(filepath.Join(t.TempDir(), "nil.wav"))
	if err != nil {
		t.Fatalf("create output: %v", err)
	}
	defer out.Close()

	enc := NewEncoder(out, 44100, 16, 1, wavFormatPCM)
	err = enc.Write(nil)
	if err == nil || !strings.Contains(err.Error(), "can't add a nil buffer") {
		t.Fatalf("expected nil buffer error, got %v", err)
	}
}

func TestEncoderWriteIEEEFloatInvalidBitDepth(t *testing.T) {
	out, err := os.Create(filepath.Join(t.TempDir(), "float-invalid.wav"))
	if err != nil {
		t.Fatalf("create output: %v", err)
	}
	defer out.Close()

	enc := NewEncoder(out, 44100, 16, 1, wavFormatIEEEFloat)
	buf := &audio.Float32Buffer{
		Format: &audio.Format{NumChannels: 1, SampleRate: 44100},
		Data:   []float32{0.1, -0.1},
	}

	err = enc.Write(buf)
	if err == nil || !strings.Contains(err.Error(), "unsupported float bit depth") {
		t.Fatalf("expected unsupported float bit depth error, got %v", err)
	}
}

func TestEncoderWriteIEEEFloat64RoundTrip(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "float64.wav")
	out, err := os.Create(outPath)
	if err != nil {
		t.Fatalf("create output: %v", err)
	}

	enc := NewEncoder(out, 44100, 64, 1, wavFormatIEEEFloat)
	in := &audio.Float32Buffer{
		Format: &audio.Format{NumChannels: 1, SampleRate: 44100},
		Data:   []float32{-1.2, -0.5, 0.0, 0.5, 1.2},
	}

	if err := enc.Write(in); err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	if err := enc.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close output: %v", err)
	}

	f, err := os.Open(outPath)
	if err != nil {
		t.Fatalf("open encoded file: %v", err)
	}
	defer f.Close()

	dec := NewDecoder(f)
	buf, err := dec.FullPCMBuffer()
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if int(dec.WavAudioFormat) != wavFormatIEEEFloat {
		t.Fatalf("expected float wav format, got %d", dec.WavAudioFormat)
	}

	if dec.BitDepth != 64 {
		t.Fatalf("expected bit depth 64, got %d", dec.BitDepth)
	}

	if len(buf.Data) != len(in.Data) {
		t.Fatalf("expected %d samples, got %d", len(in.Data), len(buf.Data))
	}

	expected := []float32{-1, -0.5, 0, 0.5, 1}
	for i := range expected {
		if !float32ApproxEqual(buf.Data[i], expected[i], 1e-6) {
			t.Fatalf("sample %d mismatch, expected %.6f got %.6f", i, expected[i], buf.Data[i])
		}
	}
}

func TestEncoderWriteFrameFloat64IEEE64(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "frame_float64_ieee.wav")
	out, err := os.Create(outPath)
	if err != nil {
		t.Fatalf("create output: %v", err)
	}

	enc := NewEncoder(out, 48000, 64, 1, wavFormatIEEEFloat)
	if err := enc.WriteFrame(float64(0.25)); err != nil {
		t.Fatalf("WriteFrame failed: %v", err)
	}

	if err := enc.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if err := out.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}

	in, err := os.Open(outPath)
	if err != nil {
		t.Fatalf("open encoded file: %v", err)
	}
	defer in.Close()

	dec := NewDecoder(in)
	buf, err := dec.FullPCMBuffer()
	if err != nil {
		t.Fatalf("decode PCM buffer: %v", err)
	}

	if dec.BitDepth != 64 {
		t.Fatalf("expected bit depth 64, got %d", dec.BitDepth)
	}

	if len(buf.Data) != 1 {
		t.Fatalf("expected one sample, got %d", len(buf.Data))
	}

	if !float32ApproxEqual(buf.Data[0], 0.25, 1e-6) {
		t.Fatalf("decoded sample=%f, want ~0.25", buf.Data[0])
	}
}

func TestEncoderWriteG711RoundTrip(t *testing.T) {
	testCases := []struct {
		name   string
		format int
	}{
		{name: "alaw", format: wavFormatALaw},
		{name: "mulaw", format: wavFormatMuLaw},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			outPath := filepath.Join(t.TempDir(), testCase.name+".wav")
			out, err := os.Create(outPath)
			if err != nil {
				t.Fatalf("create output: %v", err)
			}

			enc := NewEncoder(out, 8000, 8, 1, testCase.format)
			in := &audio.Float32Buffer{
				Format: &audio.Format{NumChannels: 1, SampleRate: 8000},
				Data:   []float32{-0.9, -0.3, 0.0, 0.3, 0.9},
			}

			if err := enc.Write(in); err != nil {
				t.Fatalf("encode failed: %v", err)
			}
			if err := enc.Close(); err != nil {
				t.Fatalf("close failed: %v", err)
			}
			if err := out.Close(); err != nil {
				t.Fatalf("close output: %v", err)
			}

			f, err := os.Open(outPath)
			if err != nil {
				t.Fatalf("open encoded file: %v", err)
			}
			defer f.Close()

			dec := NewDecoder(f)
			buf, err := dec.FullPCMBuffer()
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			if int(dec.WavAudioFormat) != testCase.format {
				t.Fatalf("expected format %d, got %d", testCase.format, dec.WavAudioFormat)
			}

			if len(buf.Data) != len(in.Data) {
				t.Fatalf("expected %d samples, got %d", len(in.Data), len(buf.Data))
			}

			for i := range in.Data {
				if !float32ApproxEqual(buf.Data[i], in.Data[i], 0.12) {
					t.Fatalf("sample %d mismatch, expected %.3f got %.3f", i, in.Data[i], buf.Data[i])
				}
			}
		})
	}
}
