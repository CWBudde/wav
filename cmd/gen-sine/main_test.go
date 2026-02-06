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
