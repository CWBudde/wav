package wav

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-audio/audio"
)

var kickSamples = []int{
	0, 0, 0, 0, 0, 0, 3, 3, 28, 28, 130, 130, 436, 436, 1103, 1103, 2140, 2140, 3073, 3073,
	2884, 2884, 760, 760, -2755, -2755, -5182, -5182, -3860, -3860, 1048, 1048, 5303, 5303,
	3885, 3885, -3378, -3378, -9971, -9971, -8119, -8119, 2616, 2616, 13344, 13344, 13297,
	13297, 553, 553, -15013, -15013, -20341, -20341, -10692, -10692, 6553, 6553, 18819, 18819,
	18824, 18824, 8617, 8617, -4253, -4253, -13305, -13305, -16289, -16289, -13913, -13913,
	-7552, -7552, 1334, 1334, 10383, 10383, 16409, 16409, 16928, 16928, 11771, 11771, 3121,
	3121, -5908, -5908, -12829, -12829, -16321, -16321, -15990, -15990, -12025, -12025, -5273,
	-5273, 2732, 2732, 10094, 10094, 15172, 15172, 17038, 17038, 15563, 15563, 11232, 11232,
	4973, 4971, -2044, -2044, -8602, -8602, -13659, -13659, -16458, -16458, -16574, -16575,
	-14012, -14012, -9294, -9294, -3352, -3352, 2823, 2823, 8485, 8485, 13125, 13125, 16228,
	16228, 17214, 17214, 15766, 15766, 12188, 12188, 7355, 7355, 2152, 2152, -2973, -2973,
	-7929, -7929, -12446, -12446, -15806, -15806, -17161, -17161, -16200, -16200, -13407,
	-13407, -9681, -9681, -5659, -5659, -1418, -1418, 3212, 3212, 8092, 8092, 12567, 12567,
	15766, 15766, 17123, 17123, 16665, 16665, 14863, 14863, 12262, 12262, 9171, 9171, 5644,
	5644, 1636, 1636, -2768, -2768, -7262, -7262, -11344, -11344, -14486, -14486, -16310,
	-16310, -16710, -16710, -15861, -15861, -14093, -14093, -11737, -11737, -8974, -8974,
	-5840, -5840, -2309, -2309, 1577, 1577, 5631, 5631, 9510, 9510, 12821, 12821, 15218,
	15218, 16500, 16500, 16663, 16663, 15861, 15861, 14338, 14338, 12322, 12322, 9960, 9960,
}

func TestDecoderSeek(t *testing.T) {
	file, err := os.Open("fixtures/bass.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	decoder := NewDecoder(file)
	// Move read cursor to the middle of the file
	// Using whence=0 should be os.SEEK_SET for go<=1.6.x else io.SeekStart
	cur, err := decoder.Seek(decoder.PCMLen()/2, 0)
	if err != nil {
		t.Fatal(err)
	}

	if cur != decoder.PCMLen()/2 {
		t.Fatal("Read cursor no in the expected position")
	}
}

func TestDecoderRewind(t *testing.T) {
	file, err := os.Open("fixtures/bass.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	decoder := NewDecoder(file)
	decoder.ReadInfo()
	buf := &audio.Float32Buffer{Format: decoder.Format(), Data: make([]float32, 512)}

	num, err := decoder.PCMBuffer(buf)
	if err != nil {
		t.Fatal(err)
	}

	if num != 512 {
		t.Fatalf("expected to read 512 samples but got %d", num)
	}

	err = decoder.Rewind()
	if err != nil {
		t.Fatal(err)
	}

	newBuf := &audio.Float32Buffer{Format: decoder.Format(), Data: make([]float32, 512)}

	num, err = decoder.PCMBuffer(newBuf)
	if err != nil {
		t.Fatal(err)
	}

	if num != 512 {
		t.Fatalf("expected to read 512 samples but got %d", num)
	}

	assertFloat32SlicesClose(t, buf.Data, newBuf.Data, 1e-6)
}

func TestDecoder_Duration(t *testing.T) {
	testCases := []struct {
		in       string
		duration time.Duration
	}{
		{"fixtures/kick.wav", 204172335 * time.Nanosecond},
	}

	for _, testCase := range testCases {
		file, err := os.Open(testCase.in)
		if err != nil {
			t.Fatal(err)
		}

		dur, err := NewDecoder(file).Duration()
		if err != nil {
			t.Fatal(err)
		}

		err = file.Close()
		if err != nil {
			t.Fatal(err)
		}

		if dur != testCase.duration {
			t.Fatalf("expected duration to be: %s but was %s", testCase.duration, dur)
		}
	}
}

func TestDecoder_IsValidFile(t *testing.T) {
	testCases := []struct {
		in      string
		isValid bool
	}{
		{"fixtures/kick.wav", true},
		{"fixtures/bass.wav", true},
		{"fixtures/dirty-kick-24b441k.wav", true},
		{"fixtures/sample.avi", false},
		{"fixtures/bloop.aif", false},
		{"fixtures/bwf.wav", true},
		// M1F1 AFsp test files
		{"fixtures/M1F1-uint8-AFsp.wav", true},
		{"fixtures/M1F1-int16-AFsp.wav", true},
		{"fixtures/M1F1-int24-AFsp.wav", true},
		{"fixtures/M1F1-int32-AFsp.wav", true},
		{"fixtures/M1F1-float32-AFsp.wav", true},
		{"fixtures/M1F1-float64-AFsp.wav", true},
		{"fixtures/M1F1-Alaw-AFsp.wav", true},
		{"fixtures/M1F1-AlawWE-AFsp.wav", true},
		{"fixtures/M1F1-mulaw-AFsp.wav", true},
		{"fixtures/M1F1-mulawWE-AFsp.wav", true},
		// Stereo test files
		{"fixtures/stereol.wav", true},
		{"fixtures/stereofl.wav", true},
		// Edge case files
		{"fixtures/GLASS.WAV", true},
		{"fixtures/Utopia-Critical-Stop.wav", true},
		// Known-valid but special-case files
		{"fixtures/M1F1-int12-AFsp.wav", true},
		{"fixtures/Pmiscck.wav", true},
		{"fixtures/Ptjunk.wav", true},
		{"fixtures/addf8-GSM-GW.wav", true},
		{"fixtures/truspech.wav", true},
		{"fixtures/voxware.wav", true},
	}

	for _, testCase := range testCases {
		file, err := os.Open(testCase.in)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()

		d := NewDecoder(file)
		if d.IsValidFile() != testCase.isValid {
			t.Fatalf("validation of the wav files doesn't match expected %t, got %t", testCase.isValid, d.IsValidFile())
		}
	}
}

func TestDecoder_G711FullPCMBuffer(t *testing.T) {
	testCases := []struct {
		input        string
		format       uint16
		sampleRate   int
		numChannels  int
		sourceBitDep int
	}{
		{
			input:        "fixtures/M1F1-Alaw-AFsp.wav",
			format:       6, // wavFormatALaw
			sampleRate:   8000,
			numChannels:  2,
			sourceBitDep: 8,
		},
		{
			input:        "fixtures/M1F1-mulaw-AFsp.wav",
			format:       7, // wavFormatMuLaw
			sampleRate:   8000,
			numChannels:  2,
			sourceBitDep: 8,
		},
		{
			input:        "fixtures/M1F1-AlawWE-AFsp.wav",
			format:       6, // wavFormatALaw
			sampleRate:   8000,
			numChannels:  2,
			sourceBitDep: 8,
		},
		{
			input:        "fixtures/M1F1-mulawWE-AFsp.wav",
			format:       7, // wavFormatMuLaw
			sampleRate:   8000,
			numChannels:  2,
			sourceBitDep: 8,
		},
		{
			input:        "fixtures/addf8-Alaw-GW.wav",
			format:       6, // wavFormatALaw
			sampleRate:   8000,
			numChannels:  1,
			sourceBitDep: 8,
		},
		{
			input:        "fixtures/addf8-mulaw-GW.wav",
			format:       7, // wavFormatMuLaw
			sampleRate:   8000,
			numChannels:  1,
			sourceBitDep: 8,
		},
	}

	for _, testCase := range testCases {
		t.Run(filepath.Base(testCase.input), func(t *testing.T) {
			file, err := os.Open(testCase.input)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()

			decoder := NewDecoder(file)

			buf, err := decoder.FullPCMBuffer()
			if err != nil {
				t.Fatalf("failed to decode %s: %v", testCase.input, err)
			}

			if int(decoder.WavAudioFormat) != int(testCase.format) {
				t.Fatalf("expected wav format %d, got %d", testCase.format, decoder.WavAudioFormat)
			}

			if buf.SourceBitDepth != testCase.sourceBitDep {
				t.Fatalf("expected source bit depth %d, got %d", testCase.sourceBitDep, buf.SourceBitDepth)
			}

			if buf.Format.SampleRate != testCase.sampleRate {
				t.Fatalf("expected sample rate %d, got %d", testCase.sampleRate, buf.Format.SampleRate)
			}

			if buf.Format.NumChannels != testCase.numChannels {
				t.Fatalf("expected channels %d, got %d", testCase.numChannels, buf.Format.NumChannels)
			}

			if len(buf.Data) == 0 {
				t.Fatalf("expected decoded samples for %s", testCase.input)
			}
		})
	}
}

func TestReadContent(t *testing.T) {
	testCases := []struct {
		input string
		total int64
		err   error
	}{
		{"fixtures/kick.wav", 180896, nil},
		{"fixtures/bwf.wav", 1003870765, nil},
	}

	for _, testCase := range testCases {
		t.Run(testCase.input, func(t *testing.T) {
			file, err := os.Open(testCase.input)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()

			d := NewDecoder(file)

			total, err := totaledDecoder(d)
			if !errors.Is(err, testCase.err) {
				t.Errorf("Expected err to be %v but got %v", testCase.err, err)
			}

			if total != testCase.total {
				t.Errorf("Expected total to be %d but got %d", testCase.total, total)
			}
		})
	}
}

func TestDecoder_Attributes(t *testing.T) {
	testCases := []struct {
		in             string
		numChannels    int
		sampleRate     int
		avgBytesPerSec int
		bitDepth       int
	}{
		{
			in:             "fixtures/kick.wav",
			numChannels:    1,
			sampleRate:     22050,
			avgBytesPerSec: 44100,
			bitDepth:       16,
		},
	}

	for _, testCase := range testCases {
		file, err := os.Open(testCase.in)
		if err != nil {
			t.Fatal(err)
		}

		decoder := NewDecoder(file)
		decoder.ReadInfo()
		file.Close()

		if int(decoder.NumChans) != testCase.numChannels {
			t.Fatalf("expected info to have %d channels but it has %d", testCase.numChannels, decoder.NumChans)
		}

		if int(decoder.SampleRate) != testCase.sampleRate {
			t.Fatalf("expected info to have a sample rate of %d but it has %d", testCase.sampleRate, decoder.SampleRate)
		}

		if int(decoder.AvgBytesPerSec) != testCase.avgBytesPerSec {
			t.Fatalf("expected info to have %d avg bytes per sec but it has %d", testCase.avgBytesPerSec, decoder.AvgBytesPerSec)
		}

		if int(decoder.BitDepth) != testCase.bitDepth {
			t.Fatalf("expected info to have %d bits per sample but it has %d", testCase.bitDepth, decoder.BitDepth)
		}
	}
}

func TestDecoderMisalignedInstChunk(t *testing.T) {
	file, err := os.Open("fixtures/misaligned-chunk.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	d := NewDecoder(file)
	floatBuf := make([]float32, 255)

	buf := &audio.Float32Buffer{Data: floatBuf}

	_, err = d.PCMBuffer(buf)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDecoder_PCMBuffer(t *testing.T) {
	testCases := []struct {
		input            string
		desc             string
		bitDepth         int
		samples          []int
		samplesAvailable int
	}{
		{
			"fixtures/bass.wav",
			"bass.wav 2 ch,  44100 Hz, 24-bit little-endian signed integer",
			24,
			[]int{
				0, 0, 110, 103, 63, 58, -2915, -2756, 2330, 2209, 8443, 8009, -1199, -1062,
				-2373, -2101, -6344, -5771, -17792, -16537, -64843, -61110, -82618, -78260,
				-24782, -24011, 111633, 104295, 235773, 221196, 275505,
			},
			47914,
		},
		{
			"fixtures/kick-16b441k.wav",
			"kick-16b441k.wav 2 ch,  44100 Hz, '16-bit little-endian signed integer",
			16,
			kickSamples,
			15564,
		},
		{
			"fixtures/padded24b.wav",
			"24b padded wav file",
			24,
			nil,
			3713,
		},
		{
			"fixtures/listChunkInHeader.wav",
			"LIST chunk before the PCM data",
			24,
			nil,
			30636,
		},
		// M1F1 AFsp test files
		{
			"fixtures/M1F1-uint8-AFsp.wav",
			"M1F1-uint8-AFsp.wav 2 ch, 8000 Hz, 8-bit unsigned integer",
			8,
			nil,
			46986,
		},
		{
			"fixtures/M1F1-int16-AFsp.wav",
			"M1F1-int16-AFsp.wav 2 ch, 8000 Hz, 16-bit signed integer",
			16,
			nil,
			46986,
		},
		{
			"fixtures/M1F1-int12-AFsp.wav",
			"M1F1-int12-AFsp.wav 2 ch, 8000 Hz, 12/16-bit signed integer",
			12,
			nil,
			46986,
		},
		{
			"fixtures/M1F1-int24-AFsp.wav",
			"M1F1-int24-AFsp.wav 2 ch, 8000 Hz, 24-bit signed integer",
			24,
			nil,
			46986,
		},
		{
			"fixtures/M1F1-int32-AFsp.wav",
			"M1F1-int32-AFsp.wav 2 ch, 8000 Hz, 32-bit signed integer",
			32,
			nil,
			46986,
		},
		{
			"fixtures/M1F1-float32-AFsp.wav",
			"M1F1-float32-AFsp.wav 2 ch, 8000 Hz, 32-bit IEEE float",
			32,
			nil,
			46986,
		},
		{
			"fixtures/M1F1-float64-AFsp.wav",
			"M1F1-float64-AFsp.wav 2 ch, 8000 Hz, 64-bit IEEE float",
			64,
			nil,
			46986,
		},
		// Stereo test files
		{
			"fixtures/stereol.wav",
			"stereol.wav 2 ch, 22050 Hz, 16-bit signed integer",
			16,
			nil,
			58032,
		},
		{
			"fixtures/stereofl.wav",
			"stereofl.wav 2 ch, 22050 Hz, 32-bit IEEE float",
			32,
			nil,
			58032,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			path, _ := filepath.Abs(testCase.input)

			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()

			decoder := NewDecoder(file)

			samples := []float32{}
			floatBuf := make([]float32, 255)
			buf := &audio.Float32Buffer{Data: floatBuf}

			var (
				samplesAvailable int
				numSamples       int
			)

			for err == nil {
				numSamples, err = decoder.PCMBuffer(buf)
				if err != nil {
					t.Fatal(err)
				}

				if numSamples == 0 {
					break
				}

				samplesAvailable += numSamples

				samples = append(samples, buf.Data...)
			}

			if samplesAvailable != testCase.samplesAvailable {
				t.Fatalf("expected %d samples available, got %d", testCase.samplesAvailable, samplesAvailable)
			}

			if buf.SourceBitDepth != testCase.bitDepth {
				t.Fatalf("expected source bit depth to be %d but got %d", testCase.bitDepth, buf.SourceBitDepth)
			}

			// allow to test the first samples of the content
			if testCase.samples != nil {
				for i, sample := range samples {
					if i >= len(testCase.samples) {
						break
					}

					expected := normalizePCMInt(testCase.samples[i], testCase.bitDepth)
					if !float32ApproxEqual(sample, expected, 1e-5) {
						t.Fatalf("Expected %.6f at position %d, but got %.6f", expected, i, sample)
					}
				}
			}
		})
	}
}

func TestDecoder_Int12MatchesInt16Fixture(t *testing.T) {
	int12File, err := os.Open("fixtures/M1F1-int12-AFsp.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer int12File.Close()

	int16File, err := os.Open("fixtures/M1F1-int16-AFsp.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer int16File.Close()

	int12Buf, err := NewDecoder(int12File).FullPCMBuffer()
	if err != nil {
		t.Fatalf("failed decoding int12 fixture: %v", err)
	}

	int16Buf, err := NewDecoder(int16File).FullPCMBuffer()
	if err != nil {
		t.Fatalf("failed decoding int16 fixture: %v", err)
	}

	if len(int12Buf.Data) != len(int16Buf.Data) {
		t.Fatalf("expected matching sample counts, got %d and %d", len(int12Buf.Data), len(int16Buf.Data))
	}

	for i := range int12Buf.Data {
		if !float32ApproxEqual(int12Buf.Data[i], int16Buf.Data[i], 1e-6) {
			t.Fatalf("sample %d mismatch: int12 %.6f != int16 %.6f", i, int12Buf.Data[i], int16Buf.Data[i])
		}
	}
}

func TestDecoder_UnsupportedCompressedFormats(t *testing.T) {
	testCases := []struct {
		path       string
		formatCode uint16
		formatName string
	}{
		{path: "fixtures/truspech.wav", formatCode: 34, formatName: "TrueSpeech"},
		{path: "fixtures/voxware.wav", formatCode: 6172, formatName: "Voxware"},
	}

	for _, testCase := range testCases {
		t.Run(filepath.Base(testCase.path), func(t *testing.T) {
			file, err := os.Open(testCase.path)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()

			dec := NewDecoder(file)

			// File structure is valid WAV even though codec is unsupported
			if !dec.IsValidFile() {
				t.Fatalf("expected %s to be a valid WAV file", testCase.path)
			}

			if dec.WavAudioFormat != testCase.formatCode {
				t.Fatalf("expected format %d, got %d", testCase.formatCode, dec.WavAudioFormat)
			}

			// FullPCMBuffer must return ErrUnsupportedCompressedFormat
			_, err = dec.FullPCMBuffer()
			if !errors.Is(err, ErrUnsupportedCompressedFormat) {
				t.Fatalf("FullPCMBuffer: expected ErrUnsupportedCompressedFormat, got %v", err)
			}

			// PCMBuffer must also return ErrUnsupportedCompressedFormat
			err = dec.Rewind()
			if err != nil {
				t.Fatalf("rewind failed: %v", err)
			}

			buf := &audio.Float32Buffer{
				Format: &audio.Format{
					NumChannels: int(dec.NumChans),
					SampleRate:  int(dec.SampleRate),
				},
				Data: make([]float32, 2048),
			}

			_, err = dec.PCMBuffer(buf)
			if !errors.Is(err, ErrUnsupportedCompressedFormat) {
				t.Fatalf("PCMBuffer: expected ErrUnsupportedCompressedFormat, got %v", err)
			}
		})
	}
}

func TestDecoder_FullPCMBuffer(t *testing.T) {
	testCases := []struct {
		input       string
		desc        string
		samples     []int
		numSamples  int
		totalFrames int
		numChannels int
		sampleRate  int
		bitDepth    int
	}{
		{
			"fixtures/bass.wav",
			"2 ch,  44100 Hz, 'lpcm' 24-bit little-endian signed integer",
			[]int{
				0, 0, 110, 103, 63, 58, -2915, -2756, 2330, 2209, 8443, 8009, -1199, -1062,
				-2373, -2101, -6344, -5771, -17792, -16537, -64843, -61110, -82618, -78260,
				-24782, -24011, 111633, 104295, 235773, 221196, 275505,
			},
			47914,
			23957,
			2,
			44100,
			24,
		},
		{
			"fixtures/kick-16b441k.wav",
			"2 ch,  44100 Hz, 'lpcm' (0x0000000C) 16-bit little-endian signed integer",
			kickSamples,
			15564,
			7782,
			2,
			44100,
			16,
		},
		{
			"fixtures/kick.wav",
			"1 ch,  22050 Hz, 'lpcm' 16-bit little-endian signed integer",
			[]int{
				76, 75, 77, 73, 74, 69, 73, 68, 72, 66, 67, 71, 529, 1427, 2243, 2943, 3512, 3953,
				4258, 4436, 4486, 4412, 4220, 3901, 3476, 2937, 2294, 1555, 709, -212, -1231, -2322,
			},
			4484,
			4484,
			1,
			22050,
			16,
		},
		// M1F1 AFsp test files
		{
			"fixtures/M1F1-uint8-AFsp.wav",
			"2 ch, 8000 Hz, 8-bit unsigned integer",
			nil,
			46986,
			23493,
			2,
			8000,
			8,
		},
		{
			"fixtures/M1F1-int16-AFsp.wav",
			"2 ch, 8000 Hz, 16-bit signed integer",
			nil,
			46986,
			23493,
			2,
			8000,
			16,
		},
		{
			"fixtures/M1F1-int24-AFsp.wav",
			"2 ch, 8000 Hz, 24-bit signed integer",
			nil,
			46986,
			23493,
			2,
			8000,
			24,
		},
		{
			"fixtures/M1F1-int32-AFsp.wav",
			"2 ch, 8000 Hz, 32-bit signed integer",
			nil,
			46986,
			23493,
			2,
			8000,
			32,
		},
		{
			"fixtures/M1F1-float32-AFsp.wav",
			"2 ch, 8000 Hz, 32-bit IEEE float",
			nil,
			46986,
			23493,
			2,
			8000,
			32,
		},
		{
			"fixtures/M1F1-float64-AFsp.wav",
			"2 ch, 8000 Hz, 64-bit IEEE float",
			nil,
			46986,
			23493,
			2,
			8000,
			64,
		},
		// Stereo test files
		{
			"fixtures/stereol.wav",
			"2 ch, 22050 Hz, 16-bit signed integer",
			nil,
			58032,
			29016,
			2,
			22050,
			16,
		},
		{
			"fixtures/stereofl.wav",
			"2 ch, 22050 Hz, 32-bit IEEE float",
			nil,
			58032,
			29016,
			2,
			22050,
			32,
		},
	}

	for i, testCase := range testCases {
		t.Logf("%d - %s\n", i, testCase.input)
		path, _ := filepath.Abs(testCase.input)

		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()

		d := NewDecoder(file)

		buf, err := d.FullPCMBuffer()
		if err != nil {
			t.Fatal(err)
		}

		if len(buf.Data) != testCase.numSamples {
			t.Fatalf("the length of the buffer (%d) didn't match what we expected (%d)", len(buf.Data), testCase.numSamples)
		}

		for i := range len(testCase.samples) {
			expected := normalizePCMInt(testCase.samples[i], testCase.bitDepth)
			if !float32ApproxEqual(buf.Data[i], expected, 1e-5) {
				t.Fatalf("Expected %.6f at position %d, but got %.6f", expected, i, buf.Data[i])
			}
		}

		if buf.Format.SampleRate != testCase.sampleRate {
			t.Fatalf("expected samplerate to be %d but got %d", testCase.sampleRate, buf.Format.SampleRate)
		}

		if buf.Format.NumChannels != testCase.numChannels {
			t.Fatalf("expected channel number to be %d but got %d", testCase.numChannels, buf.Format.NumChannels)
		}

		framesNbr := buf.NumFrames()
		if framesNbr != testCase.totalFrames {
			t.Fatalf("Expected %d frames, got %d\n", testCase.totalFrames, framesNbr)
		}
	}
}

func totaledDecoder(d *Decoder) (total int64, err error) {
	format := &audio.Format{
		NumChannels: int(d.NumChans),
		SampleRate:  int(d.SampleRate),
	}

	chunkSize := 4096
	buf := &audio.Float32Buffer{Data: make([]float32, chunkSize), Format: format}

	var num int

	for err == nil {
		num, err = d.PCMBuffer(buf)
		if err != nil {
			break
		}

		if num == 0 {
			break
		}

		for i, sampleValue := range buf.Data {
			// the buffer is longer than the data we have, we are done
			if i == num {
				break
			}

			switch int(d.BitDepth) {
			case 8:
				total += int64(float32ToPCMUint8(sampleValue))
			case 16:
				total += int64(int16(float32ToPCMInt32(sampleValue, 16)))
			default:
				total += int64(float32ToPCMInt32(sampleValue, int(d.BitDepth)))
			}
		}

		if num != chunkSize {
			break
		}
	}

	if err == nil {
		err = d.Err()
	}

	return total, err
}

func float32ApproxEqual(value, expected, epsilon float32) bool {
	diff := value - expected
	if diff < 0 {
		diff = -diff
	}

	return diff <= epsilon
}

func assertFloat32SlicesClose(t *testing.T, got, expected []float32, epsilon float32) {
	t.Helper()

	if len(got) != len(expected) {
		t.Fatalf("expected %d samples but got %d", len(expected), len(got))
	}

	for i := range got {
		if !float32ApproxEqual(got[i], expected[i], epsilon) {
			t.Fatalf("expected %.6f at position %d, but got %.6f", expected[i], i, got[i])
		}
	}
}

func TestDecoder_EdgeCases(t *testing.T) {
	testCases := []struct {
		input string
		desc  string
	}{
		{
			input: "fixtures/GLASS.WAV",
			desc:  "RIFF chunk length larger than file size",
		},
		{
			input: "fixtures/Utopia-Critical-Stop.wav",
			desc:  "PCM with misformatted fact chunk",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			file, err := os.Open(testCase.input)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()

			dec := NewDecoder(file)
			// Should be able to read basic info
			dec.ReadInfo()

			err = dec.Err()
			if err != nil {
				t.Fatalf("unexpected error reading info: %v", err)
			}

			// Should be able to decode PCM data
			buf, err := dec.FullPCMBuffer()
			if err != nil {
				t.Fatalf("unexpected error reading PCM: %v", err)
			}

			if len(buf.Data) == 0 {
				t.Fatal("expected non-zero samples")
			}
		})
	}
}

func TestDecoder_NilReceiver(t *testing.T) {
	var dec *Decoder

	if dec.SampleBitDepth() != 0 {
		t.Fatal("SampleBitDepth on nil decoder should return 0")
	}

	if dec.PCMLen() != 0 {
		t.Fatal("PCMLen on nil decoder should return 0")
	}

	if !dec.EOF() {
		t.Fatal("EOF on nil decoder should return true")
	}

	if dec.WasPCMAccessed() {
		t.Fatal("WasPCMAccessed on nil decoder should return false")
	}

	if dec.Format() != nil {
		t.Fatal("Format on nil decoder should return nil")
	}

	dur, err := dec.Duration()
	if err == nil {
		t.Fatal("Duration on nil decoder should return error")
	}

	if dur != 0 {
		t.Fatalf("Duration on nil decoder should be 0, got %v", dur)
	}

	err = dec.FwdToPCM()
	if err == nil {
		t.Fatal("FwdToPCM on nil decoder should return error")
	}
}

func TestDecoder_FmtChunkExtensible(t *testing.T) {
	file, err := os.Open("fixtures/M1F1-float32WE-AFsp.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	dec := NewDecoder(file)
	dec.ReadInfo()

	err = dec.Err()
	if err != nil {
		t.Fatalf("read info failed: %v", err)
	}

	if dec.FmtChunk == nil {
		t.Fatal("expected fmt chunk to be populated")
	}

	if dec.FmtChunk.FormatTag != wavFormatExtensible {
		t.Fatalf("expected extensible format tag, got %d", dec.FmtChunk.FormatTag)
	}

	if dec.FmtChunk.Extensible == nil {
		t.Fatal("expected extensible metadata")
	}

	if dec.FmtChunk.EffectiveFormatTag() != dec.WavAudioFormat {
		t.Fatalf("effective format mismatch, fmt=%d decoder=%d", dec.FmtChunk.EffectiveFormatTag(), dec.WavAudioFormat)
	}

	if dec.FmtChunk.Extensible.ValidBitsPerSample == 0 {
		t.Fatal("expected non-zero valid bits")
	}
}

func TestEncoder_WriteExtensibleFmtChunk(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "extensible_fmt.wav")

	out, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}

	enc := NewEncoder(out, 48000, 16, 2, wavFormatPCM)
	enc.FmtChunk = &FmtChunk{
		FormatTag: wavFormatExtensible,
		Extensible: &FmtExtensible{
			ValidBitsPerSample: 16,
			ChannelMask:        0x3,
			SubFormat:          makeSubFormatGUID(wavFormatPCM),
		},
	}

	if err := enc.Write(&audio.Float32Buffer{
		Format: &audio.Format{NumChannels: 2, SampleRate: 48000},
		Data:   []float32{0, 0, 0.25, -0.25},
	}); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if err := enc.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	if err := out.Close(); err != nil {
		t.Fatalf("file close failed: %v", err)
	}

	verify, err := os.Open(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer verify.Close()

	dec := NewDecoder(verify)
	if _, err := dec.FullPCMBuffer(); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if dec.FmtChunk == nil || dec.FmtChunk.Extensible == nil {
		t.Fatal("expected extensible fmt chunk after decode")
	}

	if dec.FmtChunk.FormatTag != wavFormatExtensible {
		t.Fatalf("expected format tag 0xFFFE, got %d", dec.FmtChunk.FormatTag)
	}

	if dec.WavAudioFormat != wavFormatPCM {
		t.Fatalf("expected effective PCM format, got %d", dec.WavAudioFormat)
	}

	if dec.FmtChunk.Extensible.ChannelMask != 0x3 {
		t.Fatalf("expected channel mask 0x3, got 0x%X", dec.FmtChunk.Extensible.ChannelMask)
	}

	if dec.FmtChunk.Extensible.ValidBitsPerSample != 16 {
		t.Fatalf("expected valid bits 16, got %d", dec.FmtChunk.Extensible.ValidBitsPerSample)
	}
}

func TestFmtChunkExtensibleRoundTripPreservesFields(t *testing.T) {
	in, err := os.Open("fixtures/M1F1-float32WE-AFsp.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	dec := NewDecoder(in)

	buf, err := dec.FullPCMBuffer()
	if err != nil {
		t.Fatalf("decode input failed: %v", err)
	}

	if dec.FmtChunk == nil || dec.FmtChunk.Extensible == nil {
		t.Fatal("expected source file to include extensible fmt")
	}

	outPath := filepath.Join(t.TempDir(), "roundtrip_extensible.wav")

	out, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}

	enc := NewEncoder(out, buf.Format.SampleRate, int(dec.BitDepth), buf.Format.NumChannels, int(dec.WavAudioFormat))

	enc.FmtChunk = dec.FmtChunk.Clone()
	if err := enc.Write(buf); err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	if err := enc.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	if err := out.Close(); err != nil {
		t.Fatalf("file close failed: %v", err)
	}

	verify, err := os.Open(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer verify.Close()

	dec2 := NewDecoder(verify)
	if _, err := dec2.FullPCMBuffer(); err != nil {
		t.Fatalf("decode roundtrip failed: %v", err)
	}

	if dec2.FmtChunk == nil || dec2.FmtChunk.Extensible == nil {
		t.Fatal("expected extensible fmt after roundtrip")
	}

	if dec2.FmtChunk.FormatTag != wavFormatExtensible {
		t.Fatalf("expected roundtrip extensible format tag, got %d", dec2.FmtChunk.FormatTag)
	}

	if dec2.WavAudioFormat != dec.WavAudioFormat {
		t.Fatalf("expected effective format %d, got %d", dec.WavAudioFormat, dec2.WavAudioFormat)
	}

	if dec2.FmtChunk.Extensible.ValidBitsPerSample != dec.FmtChunk.Extensible.ValidBitsPerSample {
		t.Fatalf(
			"valid bits changed: expected %d got %d",
			dec.FmtChunk.Extensible.ValidBitsPerSample,
			dec2.FmtChunk.Extensible.ValidBitsPerSample,
		)
	}

	if dec2.FmtChunk.Extensible.ChannelMask != dec.FmtChunk.Extensible.ChannelMask {
		t.Fatalf(
			"channel mask changed: expected 0x%X got 0x%X",
			dec.FmtChunk.Extensible.ChannelMask,
			dec2.FmtChunk.Extensible.ChannelMask,
		)
	}

	if dec2.FmtChunk.Extensible.SubFormat != dec.FmtChunk.Extensible.SubFormat {
		t.Fatal("sub format GUID changed on roundtrip")
	}
}

func TestDecoder_SampleBitDepth(t *testing.T) {
	file, err := os.Open("fixtures/kick.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	d := NewDecoder(file)
	d.ReadInfo()

	if d.SampleBitDepth() != 16 {
		t.Fatalf("expected SampleBitDepth 16, got %d", d.SampleBitDepth())
	}
}

func TestDecoder_EOF(t *testing.T) {
	file, err := os.Open("fixtures/kick.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	d := NewDecoder(file)

	if d.EOF() {
		t.Fatal("EOF should be false on fresh decoder")
	}
}

func TestDecoder_Format(t *testing.T) {
	file, err := os.Open("fixtures/kick.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	dec := NewDecoder(file)
	dec.ReadInfo()

	format := dec.Format()
	if format == nil {
		t.Fatal("Format should not be nil")
	}

	if format.NumChannels != 1 {
		t.Fatalf("expected 1 channel, got %d", format.NumChannels)
	}

	if format.SampleRate != 22050 {
		t.Fatalf("expected sample rate 22050, got %d", format.SampleRate)
	}
}

func TestDecoder_Duration_NilDecoder(t *testing.T) {
	var d *Decoder

	_, err := d.Duration()
	if !errors.Is(err, ErrDurationNilPointer) {
		t.Fatalf("expected ErrDurationNilPointer, got %v", err)
	}
}

func TestDecoder_String(t *testing.T) {
	file, err := os.Open("fixtures/kick.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	d := NewDecoder(file)
	d.ReadInfo()

	s := d.String()
	if s == "" {
		t.Fatal("String should not be empty after ReadInfo")
	}
}

func TestDecoder_NextChunk(t *testing.T) {
	file, err := os.Open("fixtures/kick.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	d := NewDecoder(file)
	d.ReadInfo()

	chunk, err := d.NextChunk()
	if err != nil {
		t.Fatalf("NextChunk returned error: %v", err)
	}

	if chunk == nil {
		t.Fatal("NextChunk should return a chunk")
	}
}

func TestDecoder_PCMBuffer_NilBuffer(t *testing.T) {
	file, err := os.Open("fixtures/kick.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	d := NewDecoder(file)

	numRead, err := d.PCMBuffer(nil)
	if err != nil {
		t.Fatalf("PCMBuffer(nil) should not error, got %v", err)
	}

	if numRead != 0 {
		t.Fatalf("PCMBuffer(nil) should return 0, got %d", numRead)
	}
}

func TestDecoder_ReadMetadata_CalledTwice(t *testing.T) {
	file, err := os.Open("fixtures/listinfo.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	dec := NewDecoder(file)
	dec.ReadMetadata()

	if dec.Metadata == nil {
		t.Fatal("expected metadata after first call")
	}

	dec.ReadMetadata()

	if dec.Metadata == nil {
		t.Fatal("metadata should still be present after second call")
	}
}

func TestDecoder_ReadMetadata_FileWithoutMetadata(t *testing.T) {
	file, err := os.Open("fixtures/kick.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	d := NewDecoder(file)
	d.ReadMetadata()

	if d.Err() != nil {
		t.Fatalf("unexpected error: %v", d.Err())
	}
}

func TestDecoder_FullPCMBuffer_NilPCMChunk(t *testing.T) {
	file, err := os.Open("fixtures/kick.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	d := NewDecoder(file)
	d.ReadInfo()
	d.pcmDataAccessed = true
	d.PCMChunk = nil

	_, err = d.FullPCMBuffer()
	if !errors.Is(err, ErrPCMChunkNotFound) {
		t.Fatalf("expected ErrPCMChunkNotFound, got %v", err)
	}
}

func TestDecoder_WasPCMAccessed(t *testing.T) {
	file, err := os.Open("fixtures/kick.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	dec := NewDecoder(file)

	if dec.WasPCMAccessed() {
		t.Fatal("WasPCMAccessed should be false before accessing PCM")
	}

	_, err = dec.FullPCMBuffer()
	if err != nil {
		t.Fatal(err)
	}

	if !dec.WasPCMAccessed() {
		t.Fatal("WasPCMAccessed should be true after reading PCM")
	}
}

func TestDecoder_InvalidFileHeader(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "badwav*.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())
	defer file.Close()

	file.WriteString("NOT_RIFF_HEADER_DATA")
	file.Seek(0, 0)

	d := NewDecoder(file)

	if d.IsValidFile() {
		t.Fatal("expected invalid file for garbage data")
	}
}

func TestDecoder_Err_ReturnsNilInitially(t *testing.T) {
	file, err := os.Open("fixtures/kick.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	d := NewDecoder(file)

	if d.Err() != nil {
		t.Fatalf("expected nil error, got %v", d.Err())
	}
}

func TestDecoder_G711RoundTrip(t *testing.T) {
	testCases := []struct {
		input  string
		format uint16
	}{
		{"fixtures/M1F1-Alaw-AFsp.wav", 6},
		{"fixtures/M1F1-mulaw-AFsp.wav", 7},
	}

	os.Mkdir("testOutput", 0o777)

	for _, testCase := range testCases {
		t.Run(filepath.Base(testCase.input), func(t *testing.T) {
			in, err := os.Open(testCase.input)
			if err != nil {
				t.Fatal(err)
			}

			dec := NewDecoder(in)

			buf, err := dec.FullPCMBuffer()
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			in.Close()

			outPath := filepath.Join("testOutput", filepath.Base(testCase.input))

			out, err := os.Create(outPath)
			if err != nil {
				t.Fatal(err)
			}

			enc := NewEncoder(out, buf.Format.SampleRate, int(dec.BitDepth), buf.Format.NumChannels, int(dec.WavAudioFormat))
			if err := enc.Write(buf); err != nil {
				t.Fatal(err)
			}

			if err := enc.Close(); err != nil {
				t.Fatal(err)
			}

			out.Close()

			defer os.Remove(outPath)

			verify, err := os.Open(outPath)
			if err != nil {
				t.Fatal(err)
			}
			defer verify.Close()

			dec2 := NewDecoder(verify)

			buf2, err := dec2.FullPCMBuffer()
			if err != nil {
				t.Fatalf("re-decode failed: %v", err)
			}

			if len(buf.Data) != len(buf2.Data) {
				t.Fatalf("sample count mismatch: %d vs %d", len(buf.Data), len(buf2.Data))
			}

			for i := range buf.Data {
				if !float32ApproxEqual(buf.Data[i], buf2.Data[i], 1e-5) {
					t.Fatalf("sample %d mismatch: %f vs %f", i, buf.Data[i], buf2.Data[i])
				}
			}
		})
	}
}
