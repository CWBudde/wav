package main

import (
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
