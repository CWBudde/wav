package wav

const (
	muLawBias = 0x84
	muLawClip = 8159
	aLawClip  = 0x0FFF
)

var (
	muLawSegmentEnd = [8]int{0x3F, 0x7F, 0xFF, 0x1FF, 0x3FF, 0x7FF, 0xFFF, 0x1FFF}
	aLawSegmentEnd  = [8]int{0x1F, 0x3F, 0x7F, 0xFF, 0x1FF, 0x3FF, 0x7FF, 0xFFF}
)

func searchSegment(value int, table [8]int) int {
	for i, end := range table {
		if value <= end {
			return i
		}
	}

	return len(table)
}

func decodeMuLawSample(sample byte) int16 {
	value := ^sample
	sign := value & 0x80
	exponent := (value >> 4) & 0x07
	mantissa := value & 0x0F

	decoded := ((int(mantissa)<<3)+muLawBias)<<exponent - muLawBias
	if sign != 0 {
		decoded = -decoded
	}

	return int16(decoded)
}

func decodeALawSample(sample byte) int16 {
	value := sample ^ 0x55
	sign := value & 0x80
	exponent := (value >> 4) & 0x07
	mantissa := value & 0x0F

	decoded := int(mantissa) << 4
	switch exponent {
	case 0:
		decoded += 8
	case 1:
		decoded += 0x108
	default:
		decoded += 0x108
		decoded <<= exponent - 1
	}

	if sign == 0 {
		decoded = -decoded
	}

	return int16(decoded)
}

func encodeMuLawSample(pcm int16) byte {
	value := int(pcm) >> 2
	mask := byte(0xFF)

	if value < 0 {
		value = -value
		mask = 0x7F
	}

	if value > muLawClip {
		value = muLawClip
	}

	value += muLawBias >> 2

	segment := searchSegment(value, muLawSegmentEnd)
	if segment >= 8 {
		return 0x7F ^ mask
	}

	encoded := byte(segment<<4) | byte((value>>(segment+1))&0x0F)
	return encoded ^ mask
}

func encodeALawSample(pcm int16) byte {
	value := int(pcm) >> 3
	mask := byte(0xD5)

	if value < 0 {
		value = -value - 1
		mask = 0x55
	}

	if value > aLawClip {
		value = aLawClip
	}

	segment := searchSegment(value, aLawSegmentEnd)
	if segment >= 8 {
		return 0x7F ^ mask
	}

	encoded := byte(segment << 4)
	if segment < 2 {
		encoded |= byte((value >> 1) & 0x0F)
	} else {
		encoded |= byte((value >> segment) & 0x0F)
	}

	return encoded ^ mask
}
