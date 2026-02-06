package wav

import (
	"testing"
	"time"
)

func TestNullTermStr(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{"with null", []byte{'h', 'e', 'l', 'l', 'o', 0, 'x'}, "hello"},
		{"no null", []byte{'h', 'e', 'l', 'l', 'o'}, "hello"},
		{"empty", []byte{}, ""},
		{"only null", []byte{0}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nullTermStr(tt.in)
			if got != tt.want {
				t.Fatalf("nullTermStr(%v)=%q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestClen(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want int
	}{
		{"with null at 3", []byte{'a', 'b', 'c', 0, 'd'}, 3},
		{"no null", []byte{'a', 'b', 'c'}, 3},
		{"empty", []byte{}, 0},
		{"null first", []byte{0, 'a'}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clen(tt.in)
			if got != tt.want {
				t.Fatalf("clen(%v)=%d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestSampleDuration(t *testing.T) {
	tests := []struct {
		name       string
		sampleRate int
		want       time.Duration
	}{
		{"44100Hz", 44100, time.Second / 44100},
		{"zero", 0, 0},
		{"negative", -48000, time.Second / 48000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sampleDuration(tt.sampleRate)
			if got != tt.want {
				t.Fatalf("sampleDuration(%d)=%v, want %v", tt.sampleRate, got, tt.want)
			}
		})
	}
}

func TestBytesNumFromDuration(t *testing.T) {
	dur := time.Second

	got := bytesNumFromDuration(dur, 44100, 16)
	if got <= 0 {
		t.Fatalf("bytesNumFromDuration(1s, 44100, 16)=%d, want positive", got)
	}

	// 48000 Hz divides evenly into 1 second
	got = bytesNumFromDuration(dur, 48000, 16)
	if got != 96000 {
		t.Fatalf("bytesNumFromDuration(1s, 48000, 16)=%d, want 96000", got)
	}
}

func TestSamplesNumFromDuration(t *testing.T) {
	dur := time.Second

	got := samplesNumFromDuration(dur, 48000)
	if got != 48000 {
		t.Fatalf("samplesNumFromDuration(1s, 48000)=%d, want 48000", got)
	}
}
