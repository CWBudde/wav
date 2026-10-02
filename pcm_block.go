package wav

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Each block decoder receives matching slices with complete samples. Dispatch
// once per block to keep readers, closures, and normalization switches out of
// the sample loop.
func sampleDecodeBlockFunc(bits int, format uint16) (func([]float32, []byte), error) {
	switch format {
	case wavFormatPCM:
		return integerDecodeBlockFunc(bits)
	case wavFormatIEEEFloat:
		return floatDecodeBlockFunc(bits)
	case wavFormatALaw:
		if bits != 8 {
			return nil, fmt.Errorf("%w: %d", errUnsupportedALawBitDepth, bits)
		}

		return decodeALawBlock, nil
	case wavFormatMuLaw:
		if bits != 8 {
			return nil, fmt.Errorf("%w: %d", errUnsupportedMuLawBitDepth, bits)
		}

		return decodeMuLawBlock, nil
	default:
		return nil, fmt.Errorf("%w: %d", errUnsupportedWavFormat, format)
	}
}

func floatDecodeBlockFunc(bits int) (func([]float32, []byte), error) {
	switch bits {
	case 32:
		return decodeFloat32Block, nil
	case 64:
		return decodeFloat64Block, nil
	default:
		return nil, fmt.Errorf("%w: %d", errUnhandledFloatBitDepth, bits)
	}
}

func integerDecodeBlockFunc(bits int) (func([]float32, []byte), error) {
	switch {
	case bits == 8:
		return decodePCM8Block, nil
	case bits > 8 && bits <= 16:
		return decodePCM16Block, nil
	case bits > 16 && bits <= 24:
		return decodePCM24Block, nil
	case bits > 24 && bits <= 32:
		return decodePCM32Block, nil
	default:
		return nil, fmt.Errorf("%w: %d", errUnhandledByteDepth, bits)
	}
}

func decodePCM8Block(dst []float32, src []byte) {
	for i := range dst {
		dst[i] = float32((float64(src[i]) - floatPCM8Center) / scalePCMInt8)
	}
}

func decodePCM16Block(dst []float32, src []byte) {
	for i := range dst {
		dst[i] = float32(int16(binary.LittleEndian.Uint16(src[i*2:]))) / scalePCMInt16
	}
}

func decodePCM24Block(dst []float32, src []byte) {
	for i := range dst {
		offset := i * 3
		value := int32(src[offset]) | int32(src[offset+1])<<8 | int32(src[offset+2])<<16
		value = value << 8 >> 8
		dst[i] = float32(value) / scalePCMInt24
	}
}

func decodePCM32Block(dst []float32, src []byte) {
	for i := range dst {
		dst[i] = float32(int32(binary.LittleEndian.Uint32(src[i*4:]))) / scalePCMInt32
	}
}

func decodeFloat32Block(dst []float32, src []byte) {
	for i := range dst {
		dst[i] = math.Float32frombits(binary.LittleEndian.Uint32(src[i*4:]))
	}
}

func decodeFloat64Block(dst []float32, src []byte) {
	for i := range dst {
		dst[i] = float32(math.Float64frombits(binary.LittleEndian.Uint64(src[i*8:])))
	}
}

func decodeALawBlock(dst []float32, src []byte) {
	for i := range dst {
		dst[i] = float32(decodeALawSample(src[i])) / scalePCMInt16
	}
}

func decodeMuLawBlock(dst []float32, src []byte) {
	for i := range dst {
		dst[i] = float32(decodeMuLawSample(src[i])) / scalePCMInt16
	}
}
