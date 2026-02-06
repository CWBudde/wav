package wav

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-audio/audio"
)

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
