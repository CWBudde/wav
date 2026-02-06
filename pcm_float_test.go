package wav

import "testing"

func TestClampFloat32(t *testing.T) {
	tests := []struct {
		name     string
		value    float32
		min, max float32
		want     float32
	}{
		{"below min", -2, -1, 1, -1},
		{"at min", -1, -1, 1, -1},
		{"in range", 0.5, -1, 1, 0.5},
		{"at max", 1, -1, 1, 1},
		{"above max", 2, -1, 1, 1},
		{"zero", 0, -1, 1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clampFloat32(tt.value, tt.min, tt.max)
			if got != tt.want {
				t.Fatalf("clampFloat32(%f, %f, %f)=%f, want %f", tt.value, tt.min, tt.max, got, tt.want)
			}
		})
	}
}

func TestClampFloat64(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		min, max float64
		want     float64
	}{
		{"below min", -2, -1, 1, -1},
		{"at min", -1, -1, 1, -1},
		{"in range", 0.5, -1, 1, 0.5},
		{"at max", 1, -1, 1, 1},
		{"above max", 2, -1, 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clampFloat64(tt.value, tt.min, tt.max)
			if got != tt.want {
				t.Fatalf("clampFloat64(%f, %f, %f)=%f, want %f", tt.value, tt.min, tt.max, got, tt.want)
			}
		})
	}
}

func TestNormalizePCMInt(t *testing.T) {
	tests := []struct {
		name     string
		sample   int
		bitDepth int
		want     float32
	}{
		{"8bit center", 128, 8, 0.003921628},
		{"8bit min", 0, 8, -1},
		{"16bit max", 32767, 16, 0.999969482},
		{"16bit zero", 0, 16, 0},
		{"24bit zero", 0, 24, 0},
		{"32bit zero", 0, 32, 0},
		{"unsupported bit depth", 100, 48, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePCMInt(tt.sample, tt.bitDepth)
			if !float32ApproxEqual(got, tt.want, 1e-4) {
				t.Fatalf("normalizePCMInt(%d, %d)=%f, want %f", tt.sample, tt.bitDepth, got, tt.want)
			}
		})
	}
}

func TestFloat32ToPCMUint8(t *testing.T) {
	tests := []struct {
		name  string
		value float32
		want  uint8
	}{
		{"min clamped", -2, 0},
		{"negative one", -1, 0},
		{"zero", 0, 128},
		{"positive one", 1, 255},
		{"max clamped", 2, 255},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := float32ToPCMUint8(tt.value)
			if got != tt.want {
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
		{"16bit positive", 0.5, 16, 16384},
		{"16bit negative", -0.5, 16, -16384},
		{"24bit positive", 0.5, 24, 4194304},
		{"32bit positive", 0.5, 32, 1073741824},
		{"unsupported", 0.5, 12, 0},
		{"16bit max", 1, 16, 32767},
		{"16bit min", -1, 16, -32768},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := float32ToPCMInt32(tt.value, tt.bitDepth)
			if got != tt.want {
				t.Fatalf("float32ToPCMInt32(%f, %d)=%d, want %d", tt.value, tt.bitDepth, got, tt.want)
			}
		})
	}
}
