package wav

import "math"

const (
	wavFormatPCM       = 1
	wavFormatIEEEFloat = 3
	maxPCMInt8Unsigned = 255
	scalePCMInt8       = 127.5
	scalePCMInt16      = 32768.0
	scalePCMInt24      = 8388608.0
	scalePCMInt32      = 2147483648.0
	floatPCM8Center    = 127.5
	floatPCM8Scale     = 127.5
	maxPCMInt16        = 32767
	maxPCMInt24        = 8388607
	maxPCMInt32        = 2147483647
)

func clampFloat32(value, min, max float32) float32 {
	if value < min {
		return min
	}

	if value > max {
		return max
	}

	return value
}

func normalizePCMInt(sample int, bitDepth int) float32 {
	switch bitDepth {
	case 8:
		return float32((float64(sample) - floatPCM8Center) / scalePCMInt8)
	case 16:
		return float32(float64(sample) / scalePCMInt16)
	case 24:
		return float32(float64(sample) / scalePCMInt24)
	case 32:
		return float32(float64(sample) / scalePCMInt32)
	default:
		return 0
	}
}

func float32ToPCMUint8(value float32) uint8 {
	value = clampFloat32(value, -1, 1)

	scaled := int(math.Round(float64((value + 1.0) * floatPCM8Scale)))
	if scaled < 0 {
		return 0
	}

	if scaled > maxPCMInt8Unsigned {
		return maxPCMInt8Unsigned
	}

	return uint8(scaled)
}

func float32ToPCMInt32(value float32, bitDepth int) int32 {
	value = clampFloat32(value, -1, 1)

	switch bitDepth {
	case 16:
		sample := min(int64(math.Round(float64(value)*scalePCMInt16)), maxPCMInt16)

		if sample < -scalePCMInt16 {
			sample = -scalePCMInt16
		}

		return int32(sample)
	case 24:
		sample := min(int64(math.Round(float64(value)*scalePCMInt24)), maxPCMInt24)

		if sample < -scalePCMInt24 {
			sample = -scalePCMInt24
		}

		return int32(sample)
	case 32:
		sample := min(int64(math.Round(float64(value)*scalePCMInt32)), maxPCMInt32)

		if sample < -scalePCMInt32 {
			sample = -scalePCMInt32
		}

		return int32(sample)
	default:
		return 0
	}
}
