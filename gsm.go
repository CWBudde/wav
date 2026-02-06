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

func gsmAdd(a, b int16) int16 {
	sum := int32(a) + int32(b)
	if sum > 32767 {
		return 32767
	}

	if sum < -32768 {
		return -32768
	}

	return int16(sum)
}

func gsmSub(a, b int16) int16 {
	diff := int32(a) - int32(b)
	if diff > 32767 {
		return 32767
	}

	if diff < -32768 {
		return -32768
	}

	return int16(diff)
}

func gsmMultR(a, b int16) int16 {
	if a == -32768 && b == -32768 {
		return 32767
	}

	return int16((int32(a)*int32(b) + 16384) >> 15)
}

func gsmAbs(a int16) int16 {
	if a == -32768 {
		return 32767
	}

	if a < 0 {
		return -a
	}

	return a
}

// Signed arithmetic shift right (arithmetic, preserves sign).
func sasr(x int16, n uint) int16 {
	return int16(int32(x) >> n)
}

// gsmAsl performs arithmetic shift left with saturation.
func gsmAsl(a int16, n int16) int16 {
	if n <= 0 {
		return a
	}

	if n >= 16 {
		return 0
	}

	return int16(int32(a) << uint(n))
}

// gsmAsr performs arithmetic shift right.
func gsmAsr(a int16, n int16) int16 {
	if n <= 0 {
		return a
	}

	if n >= 16 {
		if a < 0 {
			return -1
		}

		return 0
	}

	return int16(int32(a) >> uint(n))
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
		return f1, f2, fmt.Errorf("GSM block too short: %d bytes, need %d", len(data), gsmBlockSize)
	}

	// Frame 1: bytes 0..32 (260 bits = 32.5 bytes)
	c := 0

	var sr uint16

	sr = uint16(data[c])
	c++
	f1.LAR[0] = int16(sr & 0x3f)
	sr >>= 6
	sr |= uint16(data[c]) << 2
	c++
	f1.LAR[1] = int16(sr & 0x3f)
	sr >>= 6
	sr |= uint16(data[c]) << 4
	c++
	f1.LAR[2] = int16(sr & 0x1f)
	sr >>= 5
	f1.LAR[3] = int16(sr & 0x1f)
	sr >>= 5
	sr |= uint16(data[c]) << 2
	c++
	f1.LAR[4] = int16(sr & 0xf)
	sr >>= 4
	f1.LAR[5] = int16(sr & 0xf)
	sr >>= 4
	sr |= uint16(data[c]) << 2
	c++ // byte 4
	f1.LAR[6] = int16(sr & 0x7)
	sr >>= 3
	f1.LAR[7] = int16(sr & 0x7)
	sr >>= 3

	// Subframes 0-3 for frame 1
	for s := range 4 {
		sr |= uint16(data[c]) << 4
		c++
		f1.sub[s].Nc = int16(sr & 0x7f)
		sr >>= 7
		f1.sub[s].bc = int16(sr & 0x3)
		sr >>= 2
		f1.sub[s].Mc = int16(sr & 0x3)
		sr >>= 2
		sr |= uint16(data[c]) << 1
		c++
		f1.sub[s].xmaxc = int16(sr & 0x3f)
		sr >>= 6
		f1.sub[s].xMc[0] = int16(sr & 0x7)
		sr >>= 3
		sr = uint16(data[c])
		c++
		f1.sub[s].xMc[1] = int16(sr & 0x7)
		sr >>= 3
		f1.sub[s].xMc[2] = int16(sr & 0x7)
		sr >>= 3
		sr |= uint16(data[c]) << 2
		c++
		f1.sub[s].xMc[3] = int16(sr & 0x7)
		sr >>= 3
		f1.sub[s].xMc[4] = int16(sr & 0x7)
		sr >>= 3
		f1.sub[s].xMc[5] = int16(sr & 0x7)
		sr >>= 3
		sr |= uint16(data[c]) << 1
		c++
		f1.sub[s].xMc[6] = int16(sr & 0x7)
		sr >>= 3
		f1.sub[s].xMc[7] = int16(sr & 0x7)
		sr >>= 3
		f1.sub[s].xMc[8] = int16(sr & 0x7)
		sr >>= 3
		sr = uint16(data[c])
		c++
		f1.sub[s].xMc[9] = int16(sr & 0x7)
		sr >>= 3
		f1.sub[s].xMc[10] = int16(sr & 0x7)
		sr >>= 3
		sr |= uint16(data[c]) << 2
		c++
		f1.sub[s].xMc[11] = int16(sr & 0x7)
		sr >>= 3
		f1.sub[s].xMc[12] = int16(sr & 0x7)
		sr >>= 3
	}

	// The lower 4 bits of sr carry over to frame 2.
	frameChain := sr & 0xf

	// Frame 2: bytes 33..64 (260 bits)
	sr = frameChain
	sr |= uint16(data[c]) << 4
	c++
	f2.LAR[0] = int16(sr & 0x3f)
	sr >>= 6
	f2.LAR[1] = int16(sr & 0x3f)
	sr >>= 6
	sr = uint16(data[c])
	c++
	f2.LAR[2] = int16(sr & 0x1f)
	sr >>= 5
	sr |= uint16(data[c]) << 3
	c++
	f2.LAR[3] = int16(sr & 0x1f)
	sr >>= 5
	f2.LAR[4] = int16(sr & 0xf)
	sr >>= 4
	sr |= uint16(data[c]) << 2
	c++
	f2.LAR[5] = int16(sr & 0xf)
	sr >>= 4
	f2.LAR[6] = int16(sr & 0x7)
	sr >>= 3
	f2.LAR[7] = int16(sr & 0x7)
	sr >>= 3

	// Subframes 0-3 for frame 2
	for s := range 4 {
		sr = uint16(data[c])
		c++
		f2.sub[s].Nc = int16(sr & 0x7f)
		sr >>= 7
		sr |= uint16(data[c]) << 1
		c++
		f2.sub[s].bc = int16(sr & 0x3)
		sr >>= 2
		f2.sub[s].Mc = int16(sr & 0x3)
		sr >>= 2
		sr |= uint16(data[c]) << 5
		c++
		f2.sub[s].xmaxc = int16(sr & 0x3f)
		sr >>= 6
		f2.sub[s].xMc[0] = int16(sr & 0x7)
		sr >>= 3
		f2.sub[s].xMc[1] = int16(sr & 0x7)
		sr >>= 3
		sr |= uint16(data[c]) << 1
		c++
		f2.sub[s].xMc[2] = int16(sr & 0x7)
		sr >>= 3
		f2.sub[s].xMc[3] = int16(sr & 0x7)
		sr >>= 3
		f2.sub[s].xMc[4] = int16(sr & 0x7)
		sr >>= 3
		sr = uint16(data[c])
		c++
		f2.sub[s].xMc[5] = int16(sr & 0x7)
		sr >>= 3
		f2.sub[s].xMc[6] = int16(sr & 0x7)
		sr >>= 3
		sr |= uint16(data[c]) << 2
		c++
		f2.sub[s].xMc[7] = int16(sr & 0x7)
		sr >>= 3
		f2.sub[s].xMc[8] = int16(sr & 0x7)
		sr >>= 3
		f2.sub[s].xMc[9] = int16(sr & 0x7)
		sr >>= 3
		sr |= uint16(data[c]) << 1
		c++
		f2.sub[s].xMc[10] = int16(sr & 0x7)
		sr >>= 3
		f2.sub[s].xMc[11] = int16(sr & 0x7)
		sr >>= 3
		f2.sub[s].xMc[12] = int16(sr & 0x7)
		sr >>= 3
	}

	return f1, f2, nil
}

// RPE decoding.

func apcmXmaxcToExpMant(xmaxc int16) (exp, mant int16) {
	exp = 0
	if xmaxc > 15 {
		exp = sasr(xmaxc, 3) - 1
	}

	mant = xmaxc - (exp << 3)

	if mant == 0 {
		exp = -4
		mant = 7
	} else {
		for mant <= 7 {
			mant = mant<<1 | 1
			exp--
		}

		mant -= 8
	}

	return exp, mant
}

func apcmInverseQuantize(xMc [13]int16, mant, exp int16) [13]int16 {
	var xMp [13]int16

	temp1 := gsmFAC[mant]
	temp2 := gsmSub(6, exp)
	temp3 := gsmAsl(1, gsmSub(temp2, 1))

	for i := range 13 {
		temp := (xMc[i] << 1) - 7
		temp <<= 12
		temp = gsmMultR(temp1, temp)
		temp = gsmAdd(temp, temp3)
		xMp[i] = gsmAsr(temp, temp2)
	}

	return xMp
}

func rpeGridPositioning(Mc int16, xMp [13]int16) [40]int16 {
	var ep [40]int16
	for i := range 13 {
		ep[int(Mc)+i*3] = xMp[i]
	}

	return ep
}

// Long-term synthesis filtering.

func (g *gsmDecoder) longTermSynthesis(Nc, bc int16, erp [40]int16) {
	Nr := Nc
	if Nr < 40 || Nr > 120 {
		Nr = g.nrp
	}

	g.nrp = Nr

	brp := gsmQLB[bc]

	// drp pointer is at dp0[120], so drp[k] = dp0[120+k], drp[k-Nr] = dp0[120+k-Nr]
	for k := range 40 {
		drpp := gsmMultR(brp, g.dp0[120+k-int(Nr)])
		g.dp0[120+k] = gsmAdd(erp[k], drpp)
	}

	// Shift history: dp0[-120..-1] = dp0[-80..39] in drp coordinates
	// i.e. dp0[0..119] = dp0[40..159]
	copy(g.dp0[0:120], g.dp0[40:160])
}

// Short-term synthesis filter.

func decodeLAR(LARc [8]int16) [8]int16 {
	var LARpp [8]int16

	for i := range 8 {
		temp1 := gsmAdd(LARc[i], gsmMIC[i]) << 10
		temp1 = gsmSub(temp1, gsmB[i])
		temp1 = gsmMultR(gsmINVA[i], temp1)
		LARpp[i] = gsmAdd(temp1, temp1)
	}

	return LARpp
}

func larToRp(LARp *[8]int16) {
	for i := range 8 {
		temp := LARp[i]
		if temp < 0 {
			if temp == -32768 {
				temp = 32767
			} else {
				temp = -temp
			}

			if temp < 11059 {
				LARp[i] = -(temp << 1)
			} else if temp < 20070 {
				LARp[i] = -(temp + 11059)
			} else {
				LARp[i] = -gsmAdd(sasr(temp, 2), 26112)
			}
		} else {
			if temp < 11059 {
				LARp[i] = temp << 1
			} else if temp < 20070 {
				LARp[i] = temp + 11059
			} else {
				LARp[i] = gsmAdd(sasr(temp, 2), 26112)
			}
		}
	}
}

// Short_term_synthesis_filtering: 8th-order lattice filter.
func (g *gsmDecoder) shortTermSynthFilter(rrp [8]int16, k int, wt []int16, sr []int16) {
	for j := range k {
		sri := wt[j]

		for i := 7; i >= 0; i-- {
			tmp1 := rrp[i]
			tmp2 := g.v[i]
			// GSM_MULT_R inline with saturation check
			if tmp1 == -32768 && tmp2 == -32768 {
				tmp2 = 32767
			} else {
				tmp2 = int16((int32(tmp1)*int32(tmp2) + 16384) >> 15)
			}

			sri = gsmSub(sri, tmp2)

			tmp1 = rrp[i]
			if tmp1 == -32768 && sri == -32768 {
				tmp1 = 32767
			} else {
				tmp1 = int16((int32(tmp1)*int32(sri) + 16384) >> 15)
			}

			g.v[i+1] = gsmAdd(g.v[i], tmp1)
		}

		sr[j] = sri
		g.v[0] = sri
	}
}

func (g *gsmDecoder) shortTermSynthesis(LARc [8]int16, wt [160]int16) [160]int16 {
	var s [160]int16

	LARpp_j := decodeLAR(LARc)

	// Store decoded LAR in current slot
	LARpp_j_1 := g.LARpp[g.j]
	g.j ^= 1
	g.LARpp[g.j] = LARpp_j

	// Interpolation segment 0: samples 0-12 (13 samples)
	// LARp = 3/4 * LARpp_j_1 + 1/4 * LARpp_j
	var LARp [8]int16
	for i := range 8 {
		LARp[i] = gsmAdd(sasr(LARpp_j_1[i], 2), sasr(LARpp_j[i], 2))
		LARp[i] = gsmAdd(LARp[i], sasr(LARpp_j_1[i], 1))
	}

	larToRp(&LARp)
	g.shortTermSynthFilter(LARp, 13, wt[0:13], s[0:13])

	// Interpolation segment 1: samples 13-26 (14 samples)
	// LARp = 1/2 * LARpp_j_1 + 1/2 * LARpp_j
	for i := range 8 {
		LARp[i] = gsmAdd(sasr(LARpp_j_1[i], 1), sasr(LARpp_j[i], 1))
	}

	larToRp(&LARp)
	g.shortTermSynthFilter(LARp, 14, wt[13:27], s[13:27])

	// Interpolation segment 2: samples 27-39 (13 samples)
	// LARp = 1/4 * LARpp_j_1 + 3/4 * LARpp_j
	for i := range 8 {
		LARp[i] = gsmAdd(sasr(LARpp_j_1[i], 2), sasr(LARpp_j[i], 2))
		LARp[i] = gsmAdd(LARp[i], sasr(LARpp_j[i], 1))
	}

	larToRp(&LARp)
	g.shortTermSynthFilter(LARp, 13, wt[27:40], s[27:40])

	// Interpolation segment 3: samples 40-159 (120 samples)
	// LARp = LARpp_j
	LARp = LARpp_j
	larToRp(&LARp)
	g.shortTermSynthFilter(LARp, 120, wt[40:160], s[40:160])

	return s
}

// Postprocessing: de-emphasis and truncation.
func (g *gsmDecoder) postprocess(s [160]int16) [160]int16 {
	var out [160]int16

	for k := range 160 {
		tmp := gsmMultR(g.msr, 28180)
		g.msr = gsmAdd(s[k], tmp)
		out[k] = gsmAdd(g.msr, g.msr) & ^int16(0x7)
	}

	return out
}

// decodeFrame decodes a single GSM frame (160 samples).
func (g *gsmDecoder) decodeFrame(frame *gsmFrame) [160]int16 {
	var wt [160]int16

	for s := range 4 {
		sub := &frame.sub[s]
		exp, mant := apcmXmaxcToExpMant(sub.xmaxc)
		xMp := apcmInverseQuantize(sub.xMc, mant, exp)
		erp := rpeGridPositioning(sub.Mc, xMp)

		g.longTermSynthesis(sub.Nc, sub.bc, erp)

		// Copy reconstructed signal from dp0 for this subframe.
		copy(wt[s*40:(s+1)*40], g.dp0[120:160])
	}

	s := g.shortTermSynthesis(frame.LAR, wt)

	return g.postprocess(s)
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

			return nil, fmt.Errorf("short GSM block read: %d bytes", n)
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
