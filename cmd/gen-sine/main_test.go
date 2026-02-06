package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cwbudde/wav"
)

func TestRunGeneratesWavFile(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "sine.wav")

	err := run([]string{"-output", outPath, "-length", "0.01", "-frequency", "220"})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	fi, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}

	if fi.Size() <= 44 {
		t.Fatalf("unexpected small wav file size: %d", fi.Size())
	}

	f, err := os.Open(outPath)
	if err != nil {
		t.Fatalf("open generated file: %v", err)
	}
	defer f.Close()

	dec := wav.NewDecoder(f)
	if !dec.IsValidFile() {
		t.Fatalf("generated file is not a valid wav")
	}

	if dec.SampleRate != 48000 {
		t.Fatalf("sample rate=%d, want 48000", dec.SampleRate)
	}

	if dec.BitDepth != 16 {
		t.Fatalf("bit depth=%d, want 16", dec.BitDepth)
	}

	if dec.NumChans != 1 {
		t.Fatalf("channels=%d, want 1", dec.NumChans)
	}
}

func TestRunFlagParseError(t *testing.T) {
	err := run([]string{"-length", "not-a-number"})
	if err == nil {
		t.Fatalf("expected failure for invalid flag value")
	}
}

func TestRunDefaultParams(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "default.wav")

	err := run([]string{"-output", outPath, "-length", "0.005"})
	if err != nil {
		t.Fatalf("run with defaults failed: %v", err)
	}

	f, err := os.Open(outPath)
	if err != nil {
		t.Fatalf("open generated file: %v", err)
	}
	defer f.Close()

	dec := wav.NewDecoder(f)

	buf, err := dec.FullPCMBuffer()
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// 0.005 sec * 48000 Hz = 240 samples
	if len(buf.Data) != 240 {
		t.Fatalf("expected 240 samples, got %d", len(buf.Data))
	}
}

func TestRunInvalidOutputPath(t *testing.T) {
	err := run([]string{"-output", "/nonexistent/dir/file.wav", "-length", "0.001"})
	if err == nil {
		t.Fatal("expected error for invalid output path")
	}
}
