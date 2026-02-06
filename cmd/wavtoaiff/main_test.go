package main

import (
	"bytes"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-audio/audio"
)

func TestClampFloat32(t *testing.T) {
	tests := []struct {
		name  string
		value float32
		want  float32
	}{
		{name: "below", value: -2, want: -1},
		{name: "inside", value: 0.25, want: 0.25},
		{name: "above", value: 2, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clampFloat32(tt.value, -1, 1)
			if got != tt.want {
				t.Fatalf("clampFloat32(%f)=%f, want %f", tt.value, got, tt.want)
			}
		})
	}
}

func TestFloat32ToPCMInt(t *testing.T) {
	tests := []struct {
		name     string
		value    float32
		bitDepth int
		want     int
	}{
		{name: "8bit min", value: -1, bitDepth: 8, want: 0},
		{name: "8bit max", value: 1, bitDepth: 8, want: 255},
		{name: "16bit half", value: 0.5, bitDepth: 16, want: 16384},
		{name: "24bit half", value: 0.5, bitDepth: 24, want: 4194304},
		{name: "32bit quarter", value: 0.25, bitDepth: 32, want: 536870912},
		{name: "unsupported", value: 0.5, bitDepth: 12, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := float32ToPCMInt(tt.value, tt.bitDepth)
			if got != tt.want {
				t.Fatalf("float32ToPCMInt(%f,%d)=%d, want %d", tt.value, tt.bitDepth, got, tt.want)
			}
		})
	}
}

func TestFloat32ToIntBuffer(t *testing.T) {
	format := &audio.Format{NumChannels: 1, SampleRate: 48000}
	in := []float32{-1.5, 0, 0.5, 1.5}

	got := float32ToIntBuffer(in, format, 16)
	if got.SourceBitDepth != 16 {
		t.Fatalf("unexpected bit depth %d", got.SourceBitDepth)
	}

	if got.Format != format {
		t.Fatalf("expected returned format pointer to match input")
	}

	want := []int{-32768, 0, 16384, 32767}
	if len(got.Data) != len(want) {
		t.Fatalf("unexpected data length %d", len(got.Data))
	}

	for i := range want {
		if got.Data[i] != want[i] {
			t.Fatalf("sample[%d]=%d, want %d", i, got.Data[i], want[i])
		}
	}
}

func TestFloat32ToPCMUint8(t *testing.T) {
	tests := []struct {
		name  string
		value float32
		want  uint8
	}{
		{name: "clamped low", value: -2, want: 0},
		{name: "zero maps to center", value: 0, want: 128},
		{name: "clamped high", value: 2, want: 255},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := float32ToPCMUint8(tt.value); got != tt.want {
				t.Fatalf("float32ToPCMUint8(%f)=%d, want %d", tt.value, got, tt.want)
			}
		})
	}
}

func TestFloat32ToPCMInt32(t *testing.T) {
	tests := []struct {
		name     string
		value    float32
		bitDepth int
		want     int32
	}{
		{name: "16-bit min", value: -1, bitDepth: 16, want: -32768},
		{name: "16-bit max", value: 1, bitDepth: 16, want: 32767},
		{name: "24-bit min", value: -1, bitDepth: 24, want: -8388608},
		{name: "24-bit max", value: 1, bitDepth: 24, want: 8388607},
		{name: "32-bit min", value: -1, bitDepth: 32, want: -2147483648},
		{name: "32-bit max", value: 1, bitDepth: 32, want: 2147483647},
		{name: "unsupported", value: 0.3, bitDepth: 12, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := float32ToPCMInt32(tt.value, tt.bitDepth); got != tt.want {
				t.Fatalf("float32ToPCMInt32(%f,%d)=%d, want %d", tt.value, tt.bitDepth, got, tt.want)
			}
		})
	}
}

func TestClampScaledPCMClampsMin(t *testing.T) {
	got := clampScaledPCM(-2, 32768.0, 32767)
	if got != -32768 {
		t.Fatalf("clampScaledPCM min clamp=%d, want -32768", got)
	}
}

func TestRunErrors(t *testing.T) {
	t.Run("missing path", func(t *testing.T) {
		err := run(nil, user.Current, &bytes.Buffer{})
		if !errors.Is(err, errMissingPath) {
			t.Fatalf("expected errMissingPath, got %v", err)
		}
	})

	t.Run("invalid path", func(t *testing.T) {
		err := run([]string{"-path", filepath.Join(t.TempDir(), "missing.wav")}, user.Current, &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), "invalid path") {
			t.Fatalf("expected invalid path error, got %v", err)
		}
	})

	t.Run("invalid wav", func(t *testing.T) {
		dir := t.TempDir()
		inPath := filepath.Join(dir, "notwav.bin")
		if err := os.WriteFile(inPath, []byte("not-a-wav"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		err := run([]string{"-path", inPath}, user.Current, &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), "invalid WAV file") {
			t.Fatalf("expected invalid WAV file error, got %v", err)
		}
	})
}

func TestRunConvertsFile(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "kick.wav")

	data, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "kick.wav"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	if err := os.WriteFile(inPath, data, 0o644); err != nil {
		t.Fatalf("write temp wav: %v", err)
	}

	var out bytes.Buffer
	if err := run([]string{"-path", inPath}, user.Current, &out); err != nil {
		t.Fatalf("run convert failed: %v", err)
	}

	outPath := filepath.Join(dir, "kick.aif")
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("expected output file at %s: %v", outPath, err)
	}

	if !strings.Contains(out.String(), outPath) {
		t.Fatalf("expected output message to include %q, got %q", outPath, out.String())
	}
}

func TestRunHomeExpansion(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "kick.wav")

	data, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "kick.wav"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	if err := os.WriteFile(inPath, data, 0o644); err != nil {
		t.Fatalf("write temp wav: %v", err)
	}

	// Provide a fake user.Current that returns dir as HomeDir
	fakeUser := func() (*user.User, error) {
		return &user.User{HomeDir: dir}, nil
	}

	var out bytes.Buffer
	if err := run([]string{"-path", "~/kick.wav"}, fakeUser, &out); err != nil {
		t.Fatalf("run with home expansion failed: %v", err)
	}

	outPath := filepath.Join(dir, "kick.aif")
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("expected output file at %s: %v", outPath, err)
	}
}

func TestRunUserResolutionError(t *testing.T) {
	failUser := func() (*user.User, error) {
		return nil, errors.New("no user")
	}

	err := run([]string{"-path", "/some/file.wav"}, failUser, &bytes.Buffer{})
	if !errors.Is(err, errResolveHomeDir) {
		t.Fatalf("expected errResolveHomeDir, got %v", err)
	}
}

func TestRunFlagParseError(t *testing.T) {
	err := run([]string{"-unknown-flag"}, user.Current, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}
