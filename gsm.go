package wav

// GSM 6.10 (RPE-LTP) decoder for WAV49 format.
// Pure Go port from the reference C implementation by
// Jutta Degener and Carsten Bormann, Technische Universitaet Berlin.

import (
	"errors"
	"fmt"
	"io"
)

const (
	gsmBlockSize       = 65
	gsmSamplesPerBlock = 320
	gsmSamplesPerFrame = 160
)

var (
	errGSMBlockTooShort  = errors.New("GSM block too short")
	errShortGSMBlockRead = errors.New("short GSM block read")
)

// Lookup tables from the GSM 06.10 specification.

// Table 4.3b - LTP gain quantizer levels.
var gsmQLB = [4]int16{3277, 11469, 21299, 32767}

// Table 4.6 - Normalized direct mantissa used in APCM inverse quantization.
var gsmFAC = [8]int16{18431, 20479, 22527, 24575, 26623, 28671, 30719, 32767}

// LAR decoding constants per coefficient (Table 4.1 / 4.2).
var (
	gsmB    = [8]int16{0, 0, 2048, -2560, 94, -1792, -341, -1144}              // B * 2
	gsmMIC  = [8]int16{-32, -32, -16, -16, -8, -8, -4, -4}                     // MIC
	gsmINVA = [8]int16{13107, 13107, 13107, 13107, 19223, 17476, 31454, 29708} // 1/A
)

// Fixed-point arithmetic helpers (16-bit with saturation).

func gsmAdd(left, right int16) int16 {
	sum := int32(left) + int32(right)
	if sum > 32767 {
		return 32767
	}

	if sum < -32768 {
		return -32768
	}

	return int16(sum)
}

func gsmSub(left, right int16) int16 {
	diff := int32(left) - int32(right)
	if diff > 32767 {
		return 32767
	}

	if diff < -32768 {
		return -32768
	}

	return int16(diff)
}

func gsmMultR(left, right int16) int16 {
	if left == -32768 && right == -32768 {
		return 32767
	}

	return int16((int32(left)*int32(right) + 16384) >> 15)
}

func gsmAbs(value int16) int16 {
	if value == -32768 {
		return 32767
	}

	if value < 0 {
		return -value
	}

	return value
}

// Signed arithmetic shift right (arithmetic, preserves sign).
func sasr(value int16, shiftBits uint) int16 {
	return int16(int32(value) >> shiftBits)
}

// gsmAsl performs arithmetic shift left with saturation.
func gsmAsl(value int16, shiftBits int16) int16 {
	if shiftBits <= 0 {
		return value
	}

	if shiftBits >= 16 {
		return 0
	}

	return int16(int32(value) << uint(shiftBits))
}

// gsmAsr performs arithmetic shift right.
func gsmAsr(value int16, shiftBits int16) int16 {
	if shiftBits <= 0 {
		return value
	}

	if shiftBits >= 16 {
		if value < 0 {
			return -1
		}

		return 0
	}

	return int16(int32(value) >> uint(shiftBits))
}

// GSM frame parameter structures.

type gsmSubframe struct {
	Nc    int16     // pitch lag (7 bits)
	bc    int16     // LTP gain index (2 bits)
	Mc    int16     // RPE grid position (2 bits)
	xmaxc int16     // block amplitude maximum (6 bits)
	xMc   [13]int16 // RPE pulse amplitudes (3 bits each)
}

type gsmFrame struct {
	LAR [8]int16 // log area ratio coefficients
	sub [4]gsmSubframe
}

// gsmDecoder holds persistent state for GSM 6.10 frame decoding.
type gsmDecoder struct {
	dp0   [280]int16  // long-term prediction history
	v     [9]int16    // short-term synthesis filter state
	msr   int16       // de-emphasis filter state
	nrp   int16       // last valid pitch lag
	LARpp [2][8]int16 // previous and current decoded LAR values
	j     int         // LARpp toggle index

	// Streaming state for PCMBuffer.
	leftover    []float32
	leftoverPos int
	delivered   int
	factSamples int
}

func newGSMDecoder(factSamples int) *gsmDecoder {
	return &gsmDecoder{
		nrp:         40,
		factSamples: factSamples,
	}
}

// WAV49 bit unpacking.
// Each 65-byte block contains two GSM frames packed with LSB-first bit ordering.
// Directly ported from libgsm/libsndfile gsm_decode.c WAV49 code.

func unpackWAV49Block(data []byte) (f1, f2 gsmFrame, err error) {
	if len(data) < gsmBlockSize {
		return f1, f2, fmt.Errorf("%w: %d bytes, need %d", errGSMBlockTooShort, len(data), gsmBlockSize)
	}

	// Frame 1: bytes 0..32 (260 bits = 32.5 bytes)
	byteIndex := 0

	var shiftReg uint16

	shiftReg = uint16(data[byteIndex])
	byteIndex++
	f1.LAR[0] = int16(shiftReg & 0x3f)
	shiftReg >>= 6
	shiftReg |= uint16(data[byteIndex]) << 2
	byteIndex++
	f1.LAR[1] = int16(shiftReg & 0x3f)
	shiftReg >>= 6
	shiftReg |= uint16(data[byteIndex]) << 4
	byteIndex++
	f1.LAR[2] = int16(shiftReg & 0x1f)
	shiftReg >>= 5
	f1.LAR[3] = int16(shiftReg & 0x1f)
	shiftReg >>= 5
	shiftReg |= uint16(data[byteIndex]) << 2
	byteIndex++
	f1.LAR[4] = int16(shiftReg & 0xf)
	shiftReg >>= 4
	f1.LAR[5] = int16(shiftReg & 0xf)
	shiftReg >>= 4
	shiftReg |= uint16(data[byteIndex]) << 2
	byteIndex++ // byte 4
	f1.LAR[6] = int16(shiftReg & 0x7)
	shiftReg >>= 3
	f1.LAR[7] = int16(shiftReg & 0x7)
	shiftReg >>= 3

	// Subframes 0-3 for frame 1
	for subframeIdx := range 4 {
		shiftReg |= uint16(data[byteIndex]) << 4
		byteIndex++
		f1.sub[subframeIdx].Nc = int16(shiftReg & 0x7f)
		shiftReg >>= 7
		f1.sub[subframeIdx].bc = int16(shiftReg & 0x3)
		shiftReg >>= 2
		f1.sub[subframeIdx].Mc = int16(shiftReg & 0x3)
		shiftReg >>= 2
		shiftReg |= uint16(data[byteIndex]) << 1
		byteIndex++
		f1.sub[subframeIdx].xmaxc = int16(shiftReg & 0x3f)
		shiftReg >>= 6
		f1.sub[subframeIdx].xMc[0] = int16(shiftReg & 0x7)
		shiftReg >>= 3
		shiftReg = uint16(data[byteIndex])
		byteIndex++
		f1.sub[subframeIdx].xMc[1] = int16(shiftReg & 0x7)
		shiftReg >>= 3
		f1.sub[subframeIdx].xMc[2] = int16(shiftReg & 0x7)
		shiftReg >>= 3
		shiftReg |= uint16(data[byteIndex]) << 2
		byteIndex++
		f1.sub[subframeIdx].xMc[3] = int16(shiftReg & 0x7)
		shiftReg >>= 3
		f1.sub[subframeIdx].xMc[4] = int16(shiftReg & 0x7)
		shiftReg >>= 3
		f1.sub[subframeIdx].xMc[5] = int16(shiftReg & 0x7)
		shiftReg >>= 3
		shiftReg |= uint16(data[byteIndex]) << 1
		byteIndex++
		f1.sub[subframeIdx].xMc[6] = int16(shiftReg & 0x7)
		shiftReg >>= 3
		f1.sub[subframeIdx].xMc[7] = int16(shiftReg & 0x7)
		shiftReg >>= 3
		f1.sub[subframeIdx].xMc[8] = int16(shiftReg & 0x7)
		shiftReg >>= 3
		shiftReg = uint16(data[byteIndex])
		byteIndex++
		f1.sub[subframeIdx].xMc[9] = int16(shiftReg & 0x7)
		shiftReg >>= 3
		f1.sub[subframeIdx].xMc[10] = int16(shiftReg & 0x7)
		shiftReg >>= 3
		shiftReg |= uint16(data[byteIndex]) << 2
		byteIndex++
		f1.sub[subframeIdx].xMc[11] = int16(shiftReg & 0x7)
		shiftReg >>= 3
		f1.sub[subframeIdx].xMc[12] = int16(shiftReg & 0x7)
		shiftReg >>= 3
	}

	// The lower 4 bits of sr carry over to frame 2.
	frameChain := shiftReg & 0xf

	// Frame 2: bytes 33..64 (260 bits)
	shiftReg = frameChain
	shiftReg |= uint16(data[byteIndex]) << 4
	byteIndex++
	f2.LAR[0] = int16(shiftReg & 0x3f)
	shiftReg >>= 6
	f2.LAR[1] = int16(shiftReg & 0x3f)
	shiftReg >>= 6
	shiftReg = uint16(data[byteIndex])
	byteIndex++
	f2.LAR[2] = int16(shiftReg & 0x1f)
	shiftReg >>= 5
	shiftReg |= uint16(data[byteIndex]) << 3
	byteIndex++
	f2.LAR[3] = int16(shiftReg & 0x1f)
	shiftReg >>= 5
	f2.LAR[4] = int16(shiftReg & 0xf)
	shiftReg >>= 4
	shiftReg |= uint16(data[byteIndex]) << 2
	byteIndex++
	f2.LAR[5] = int16(shiftReg & 0xf)
	shiftReg >>= 4
	f2.LAR[6] = int16(shiftReg & 0x7)
	shiftReg >>= 3
	f2.LAR[7] = int16(shiftReg & 0x7)
	shiftReg >>= 3

	// Subframes 0-3 for frame 2
	for subframeIdx := range 4 {
		shiftReg = uint16(data[byteIndex])
		byteIndex++
		f2.sub[subframeIdx].Nc = int16(shiftReg & 0x7f)
		shiftReg >>= 7
		shiftReg |= uint16(data[byteIndex]) << 1
		byteIndex++
		f2.sub[subframeIdx].bc = int16(shiftReg & 0x3)
		shiftReg >>= 2
		f2.sub[subframeIdx].Mc = int16(shiftReg & 0x3)
		shiftReg >>= 2
		shiftReg |= uint16(data[byteIndex]) << 5
		byteIndex++
		f2.sub[subframeIdx].xmaxc = int16(shiftReg & 0x3f)
		shiftReg >>= 6
		f2.sub[subframeIdx].xMc[0] = int16(shiftReg & 0x7)
		shiftReg >>= 3
		f2.sub[subframeIdx].xMc[1] = int16(shiftReg & 0x7)
		shiftReg >>= 3
		shiftReg |= uint16(data[byteIndex]) << 1
		byteIndex++
		f2.sub[subframeIdx].xMc[2] = int16(shiftReg & 0x7)
		shiftReg >>= 3
		f2.sub[subframeIdx].xMc[3] = int16(shiftReg & 0x7)
		shiftReg >>= 3
		f2.sub[subframeIdx].xMc[4] = int16(shiftReg & 0x7)
		shiftReg >>= 3
		shiftReg = uint16(data[byteIndex])
		byteIndex++
		f2.sub[subframeIdx].xMc[5] = int16(shiftReg & 0x7)
		shiftReg >>= 3
		f2.sub[subframeIdx].xMc[6] = int16(shiftReg & 0x7)
		shiftReg >>= 3
		shiftReg |= uint16(data[byteIndex]) << 2
		byteIndex++
		f2.sub[subframeIdx].xMc[7] = int16(shiftReg & 0x7)
		shiftReg >>= 3
		f2.sub[subframeIdx].xMc[8] = int16(shiftReg & 0x7)
		shiftReg >>= 3
		f2.sub[subframeIdx].xMc[9] = int16(shiftReg & 0x7)
		shiftReg >>= 3
		shiftReg |= uint16(data[byteIndex]) << 1
		byteIndex++
		f2.sub[subframeIdx].xMc[10] = int16(shiftReg & 0x7)
		shiftReg >>= 3
		f2.sub[subframeIdx].xMc[11] = int16(shiftReg & 0x7)
		shiftReg >>= 3
		f2.sub[subframeIdx].xMc[12] = int16(shiftReg & 0x7)
		shiftReg >>= 3
	}

	return f1, f2, nil
}

// RPE decoding.

func apcmXmaxcToExpMant(blockAmplitude int16) (exponent, mantissa int16) {
	exponent = 0
	if blockAmplitude > 15 {
		exponent = sasr(blockAmplitude, 3) - 1
	}

	mantissa = blockAmplitude - (exponent << 3)

	if mantissa == 0 {
		exponent = -4
		mantissa = 7
	} else {
		for mantissa <= 7 {
			mantissa = mantissa<<1 | 1
			exponent--
		}

		mantissa -= 8
	}

	return exponent, mantissa
}

func apcmInverseQuantize(quantized [13]int16, mantissa, exponent int16) [13]int16 {
	var dequantized [13]int16

	normFactor := gsmFAC[mantissa]
	shiftAmount := gsmSub(6, exponent)
	roundingOffset := gsmAsl(1, gsmSub(shiftAmount, 1))

	for pulseIdx := range 13 {
		value := (quantized[pulseIdx] << 1) - 7
		value <<= 12
		value = gsmMultR(normFactor, value)
		value = gsmAdd(value, roundingOffset)
		dequantized[pulseIdx] = gsmAsr(value, shiftAmount)
	}

	return dequantized
}

func rpeGridPositioning(gridPosition int16, pulses [13]int16) [40]int16 {
	var output [40]int16
	for pulseIdx := range 13 {
		output[int(gridPosition)+pulseIdx*3] = pulses[pulseIdx]
	}

	return output
}

// Long-term synthesis filtering.

func (g *gsmDecoder) longTermSynthesis(pitchLag, gainIndex int16, residual [40]int16) {
	validPitchLag := pitchLag
	if validPitchLag < 40 || validPitchLag > 120 {
		validPitchLag = g.nrp
	}

	g.nrp = validPitchLag

	gainCoeff := gsmQLB[gainIndex]

	// drp pointer is at dp0[120], so drp[k] = dp0[120+k], drp[k-Nr] = dp0[120+k-Nr]
	for sampleIdx := range 40 {
		predicted := gsmMultR(gainCoeff, g.dp0[120+sampleIdx-int(validPitchLag)])
		g.dp0[120+sampleIdx] = gsmAdd(residual[sampleIdx], predicted)
	}

	// Shift history: dp0[-120..-1] = dp0[-80..39] in drp coordinates
	// i.e. dp0[0..119] = dp0[40..159]
	copy(g.dp0[0:120], g.dp0[40:160])
}

// Short-term synthesis filter.

func decodeLAR(larEncoded [8]int16) [8]int16 {
	var larDecoded [8]int16

	for coeffIdx := range 8 {
		value := gsmAdd(larEncoded[coeffIdx], gsmMIC[coeffIdx]) << 10
		value = gsmSub(value, gsmB[coeffIdx])
		value = gsmMultR(gsmINVA[coeffIdx], value)
		larDecoded[coeffIdx] = gsmAdd(value, value)
	}

	return larDecoded
}

func larToRp(larParams *[8]int16) {
	for coeffIdx := range 8 {
		absValue := larParams[coeffIdx]
		if absValue < 0 {
			if absValue == -32768 {
				absValue = 32767
			} else {
				absValue = -absValue
			}

			if absValue < 11059 {
				larParams[coeffIdx] = -(absValue << 1)
			} else if absValue < 20070 {
				larParams[coeffIdx] = -(absValue + 11059)
			} else {
				larParams[coeffIdx] = -gsmAdd(sasr(absValue, 2), 26112)
			}
		} else {
			if absValue < 11059 {
				larParams[coeffIdx] = absValue << 1
			} else if absValue < 20070 {
				larParams[coeffIdx] = absValue + 11059
			} else {
				larParams[coeffIdx] = gsmAdd(sasr(absValue, 2), 26112)
			}
		}
	}
}

// Short_term_synthesis_filtering: 8th-order lattice filter.
func (g *gsmDecoder) shortTermSynthFilter(reflCoeffs [8]int16, numSamples int, input []int16, output []int16) {
	for sampleIdx := range numSamples {
		sample := input[sampleIdx]

		for coeffIdx := 7; coeffIdx >= 0; coeffIdx-- {
			coeff := reflCoeffs[coeffIdx]
			state := g.v[coeffIdx]
			// GSM_MULT_R inline with saturation check
			if coeff == -32768 && state == -32768 {
				state = 32767
			} else {
				state = int16((int32(coeff)*int32(state) + 16384) >> 15)
			}

			sample = gsmSub(sample, state)

			coeff = reflCoeffs[coeffIdx]
			if coeff == -32768 && sample == -32768 {
				coeff = 32767
			} else {
				coeff = int16((int32(coeff)*int32(sample) + 16384) >> 15)
			}

			g.v[coeffIdx+1] = gsmAdd(g.v[coeffIdx], coeff)
		}

		output[sampleIdx] = sample
		g.v[0] = sample
	}
}

func (g *gsmDecoder) shortTermSynthesis(larEncoded [8]int16, reconstructed [160]int16) [160]int16 {
	var output [160]int16

	larCurrent := decodeLAR(larEncoded)

	// Store decoded LAR in current slot
	larPrevious := g.LARpp[g.j]
	g.j ^= 1
	g.LARpp[g.j] = larCurrent

	// Interpolation segment 0: samples 0-12 (13 samples)
	// LARp = 3/4 * larPrevious + 1/4 * larCurrent
	var larInterpolated [8]int16
	for coeffIdx := range 8 {
		larInterpolated[coeffIdx] = gsmAdd(sasr(larPrevious[coeffIdx], 2), sasr(larCurrent[coeffIdx], 2))
		larInterpolated[coeffIdx] = gsmAdd(larInterpolated[coeffIdx], sasr(larPrevious[coeffIdx], 1))
	}

	larToRp(&larInterpolated)
	g.shortTermSynthFilter(larInterpolated, 13, reconstructed[0:13], output[0:13])

	// Interpolation segment 1: samples 13-26 (14 samples)
	// LARp = 1/2 * larPrevious + 1/2 * larCurrent
	for coeffIdx := range 8 {
		larInterpolated[coeffIdx] = gsmAdd(sasr(larPrevious[coeffIdx], 1), sasr(larCurrent[coeffIdx], 1))
	}

	larToRp(&larInterpolated)
	g.shortTermSynthFilter(larInterpolated, 14, reconstructed[13:27], output[13:27])

	// Interpolation segment 2: samples 27-39 (13 samples)
	// LARp = 1/4 * larPrevious + 3/4 * larCurrent
	for coeffIdx := range 8 {
		larInterpolated[coeffIdx] = gsmAdd(sasr(larPrevious[coeffIdx], 2), sasr(larCurrent[coeffIdx], 2))
		larInterpolated[coeffIdx] = gsmAdd(larInterpolated[coeffIdx], sasr(larCurrent[coeffIdx], 1))
	}

	larToRp(&larInterpolated)
	g.shortTermSynthFilter(larInterpolated, 13, reconstructed[27:40], output[27:40])

	// Interpolation segment 3: samples 40-159 (120 samples)
	// LARp = larCurrent
	larInterpolated = larCurrent
	larToRp(&larInterpolated)
	g.shortTermSynthFilter(larInterpolated, 120, reconstructed[40:160], output[40:160])

	return output
}

// Postprocessing: de-emphasis and truncation.
func (g *gsmDecoder) postprocess(input [160]int16) [160]int16 {
	var output [160]int16

	for sampleIdx := range 160 {
		deemphasis := gsmMultR(g.msr, 28180)
		g.msr = gsmAdd(input[sampleIdx], deemphasis)
		output[sampleIdx] = gsmAdd(g.msr, g.msr) & ^int16(0x7)
	}

	return output
}

// decodeFrame decodes a single GSM frame (160 samples).
func (g *gsmDecoder) decodeFrame(frame *gsmFrame) [160]int16 {
	var reconstructed [160]int16

	for subframeIdx := range 4 {
		subframe := &frame.sub[subframeIdx]
		exponent, mantissa := apcmXmaxcToExpMant(subframe.xmaxc)
		dequantized := apcmInverseQuantize(subframe.xMc, mantissa, exponent)
		residual := rpeGridPositioning(subframe.Mc, dequantized)

		g.longTermSynthesis(subframe.Nc, subframe.bc, residual)

		// Copy reconstructed signal from dp0 for this subframe.
		copy(reconstructed[subframeIdx*40:(subframeIdx+1)*40], g.dp0[120:160])
	}

	shortTermOutput := g.shortTermSynthesis(frame.LAR, reconstructed)

	return g.postprocess(shortTermOutput)
}

// decodeBlock decodes a 65-byte WAV49 block into 320 float32 samples.
func (g *gsmDecoder) decodeBlock(block []byte) ([gsmSamplesPerBlock]int16, error) {
	var out [gsmSamplesPerBlock]int16

	f1, f2, err := unpackWAV49Block(block)
	if err != nil {
		return out, err
	}

	s1 := g.decodeFrame(&f1)
	s2 := g.decodeFrame(&f2)

	copy(out[0:160], s1[:])
	copy(out[160:320], s2[:])

	return out, nil
}

// decodeAllBlocks reads all GSM blocks and returns float32 samples.
func (g *gsmDecoder) decodeAllBlocks(r io.Reader, factSamples int) ([]float32, error) {
	var allSamples []float32

	block := make([]byte, gsmBlockSize)

	for {
		n, err := io.ReadFull(r, block)
		if n == 0 || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			if n == 0 {
				break
			}
		}

		if n < gsmBlockSize {
			if errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}

			return nil, fmt.Errorf("%w: %d bytes", errShortGSMBlockRead, n)
		}

		samples, decErr := g.decodeBlock(block)
		if decErr != nil {
			return nil, decErr
		}

		for _, s := range samples {
			allSamples = append(allSamples, normalizePCMInt(int(s), 16))
		}

		if err != nil {
			break
		}
	}

	if factSamples > 0 && len(allSamples) > factSamples {
		allSamples = allSamples[:factSamples]
	}

	return allSamples, nil
}

// decodeToBuffer fills out with decoded float32 samples for streaming PCMBuffer use.
func (g *gsmDecoder) decodeToBuffer(r io.Reader, out []float32) (int, error) {
	n := 0

	// Drain leftover from previous block first.
	if g.leftoverPos < len(g.leftover) {
		avail := len(g.leftover) - g.leftoverPos

		want := len(out)
		if avail > want {
			avail = want
		}

		// Check factSamples limit.
		if g.factSamples > 0 && g.delivered+avail > g.factSamples {
			avail = g.factSamples - g.delivered
		}

		if avail <= 0 {
			return 0, nil
		}

		copy(out[:avail], g.leftover[g.leftoverPos:g.leftoverPos+avail])
		g.leftoverPos += avail
		g.delivered += avail
		n += avail

		if g.leftoverPos >= len(g.leftover) {
			g.leftover = nil
			g.leftoverPos = 0
		}
	}

	block := make([]byte, gsmBlockSize)

	for n < len(out) {
		// Check factSamples limit.
		if g.factSamples > 0 && g.delivered >= g.factSamples {
			break
		}

		nr, err := io.ReadFull(r, block)
		if nr == 0 || (errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)) && nr == 0 {
			break
		}

		if nr < gsmBlockSize {
			break
		}

		samples, decErr := g.decodeBlock(block)
		if decErr != nil {
			return n, decErr
		}

		// Convert to float32.
		var floatSamples [gsmSamplesPerBlock]float32
		for i, s := range samples {
			floatSamples[i] = normalizePCMInt(int(s), 16)
		}

		remaining := len(out) - n

		blockSamples := gsmSamplesPerBlock
		if g.factSamples > 0 && g.delivered+blockSamples > g.factSamples {
			blockSamples = g.factSamples - g.delivered
		}

		if remaining >= blockSamples {
			copy(out[n:n+blockSamples], floatSamples[:blockSamples])
			n += blockSamples
			g.delivered += blockSamples
		} else {
			copy(out[n:n+remaining], floatSamples[:remaining])
			n += remaining
			g.delivered += remaining

			// Save the rest as leftover.
			leftCount := blockSamples - remaining
			if leftCount > 0 {
				g.leftover = make([]float32, leftCount)
				copy(g.leftover, floatSamples[remaining:remaining+leftCount])
				g.leftoverPos = 0
			}
		}

		if err != nil {
			break
		}
	}

	return n, nil
}
