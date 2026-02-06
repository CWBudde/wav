package wav

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/go-audio/audio"
)

func TestEncoderRoundTrip(t *testing.T) {
	os.Mkdir("testOutput", 0o777)

	testCases := []struct {
		in       string
		out      string
		metadata *Metadata
		desc     string
	}{
		{"fixtures/kick.wav", "testOutput/kick.wav", nil, "22050 Hz @ 16 bits, 1 channel(s), 44100 avg bytes/sec, duration: 204.172335ms"},
		{"fixtures/kick-16b441k.wav", "testOutput/kick-16b441k.wav", nil, "2 ch,  44100 Hz, 'lpcm' 16-bit little-endian signed integer"},
		{"fixtures/bass.wav", "testOutput/bass.wav", nil, "44100 Hz @ 24 bits, 2 channel(s), 264600 avg bytes/sec, duration: 543.378684ms"},
		{"fixtures/8bit.wav", "testOutput/8bit.wav", &Metadata{
			Artist: "Matt", Copyright: "copyleft", Comments: "A comment", CreationDate: "2017-12-12", Engineer: "Matt A", Technician: "Matt Aimonetti",
			Genre: "test", Keywords: "go code", Medium: "Virtual", Title: "Titre", Product: "go-audio", Subject: "wav codec",
			Software: "go-audio codec", Source: "Audacity generator", Location: "Los Angeles", TrackNbr: "42",
		}, "1 ch,  44100 Hz, 8-bit unsigned integer"},
		{"fixtures/32bit.wav", "testOutput/32bit.wav", nil, "1 ch, 44100 Hz, 32-bit little-endian signed integer"},
		// IEEE Float formats
		{"fixtures/M1F1-float32-AFsp.wav", "testOutput/M1F1-float32-AFsp.wav", nil, "2 ch, 8000 Hz, 32-bit IEEE float"},
		{"fixtures/M1F1-float64-AFsp.wav", "testOutput/M1F1-float64-AFsp.wav", nil, "2 ch, 8000 Hz, 64-bit IEEE float"},
		{"fixtures/stereofl.wav", "testOutput/stereofl.wav", nil, "2 ch, 22050 Hz, 32-bit IEEE float"},
	}

	for _, testCase := range testCases {
		t.Run(path.Base(testCase.in), func(t *testing.T) {
			in, err := os.Open(testCase.in)
			if err != nil {
				t.Fatalf("couldn't open %s %v", testCase.in, err)
			}

			decdr := NewDecoder(in)

			buf, err := decdr.FullPCMBuffer()
			if err != nil {
				t.Fatalf("couldn't read buffer %s %v", testCase.in, err)
			}

			in.Close()

			out, err := os.Create(testCase.out)
			if err != nil {
				t.Fatalf("couldn't create %s %v", testCase.out, err)
			}

			encoder := NewEncoder(out,
				buf.Format.SampleRate,
				int(decdr.BitDepth),
				buf.Format.NumChannels,
				int(decdr.WavAudioFormat))
			if err = encoder.Write(buf); err != nil {
				t.Fatal(err)
			}

			if testCase.metadata != nil {
				encoder.Metadata = testCase.metadata
			}

			if err = encoder.Close(); err != nil {
				t.Fatal(err)
			}

			out.Close()

			newFile, err := os.Open(testCase.out)
			if err != nil {
				t.Fatal(err)
			}

			decoder := NewDecoder(newFile)

			nBuf, err := decoder.FullPCMBuffer()
			if err != nil {
				t.Fatalf("couldn't extract the PCM from %s - %v", newFile.Name(), err)
			}

			if testCase.metadata != nil {
				decoder.ReadMetadata()

				if decoder.Metadata == nil {
					t.Errorf("expected some metadata, got a nil value")
				}

				if testCase.metadata.Artist != decoder.Metadata.Artist {
					t.Errorf("expected Artist to be %s, but was %s", testCase.metadata.Artist, decoder.Metadata.Artist)
				}

				if testCase.metadata.Comments != decoder.Metadata.Comments {
					t.Errorf("expected Comments to be %s, but was %s", testCase.metadata.Comments, decoder.Metadata.Comments)
				}

				if testCase.metadata.Copyright != decoder.Metadata.Copyright {
					t.Errorf("expected Copyright to be %s, but was %s", testCase.metadata.Copyright, decoder.Metadata.Copyright)
				}

				if testCase.metadata.CreationDate != decoder.Metadata.CreationDate {
					t.Errorf("expected CreationDate to be %s, but was %s", testCase.metadata.CreationDate, decoder.Metadata.CreationDate)
				}

				if testCase.metadata.Engineer != decoder.Metadata.Engineer {
					t.Errorf("expected Engineer to be %s, but was %s", testCase.metadata.Engineer, decoder.Metadata.Engineer)
				}

				if testCase.metadata.Technician != decoder.Metadata.Technician {
					t.Errorf("expected Technician to be %s, but was %s", testCase.metadata.Technician, decoder.Metadata.Technician)
				}

				if testCase.metadata.Genre != decoder.Metadata.Genre {
					t.Errorf("expected Genre to be %s, but was %s", testCase.metadata.Genre, decoder.Metadata.Genre)
				}

				if testCase.metadata.Keywords != decoder.Metadata.Keywords {
					t.Errorf("expected Keywords to be %s, but was %s", testCase.metadata.Keywords, decoder.Metadata.Keywords)
				}

				if testCase.metadata.Medium != decoder.Metadata.Medium {
					t.Errorf("expected Medium to be %s, but was %s", testCase.metadata.Medium, decoder.Metadata.Medium)
				}

				if testCase.metadata.Title != decoder.Metadata.Title {
					t.Errorf("expected Title to be %s, but was %s", testCase.metadata.Title, decoder.Metadata.Title)
				}

				if testCase.metadata.Product != decoder.Metadata.Product {
					t.Errorf("expected Product to be %s, but was %s", testCase.metadata.Product, decoder.Metadata.Product)
				}

				if testCase.metadata.Subject != decoder.Metadata.Subject {
					t.Errorf("expected Subject to be %s, but was %s", testCase.metadata.Subject, decoder.Metadata.Subject)
				}

				if testCase.metadata.Software != decoder.Metadata.Software {
					t.Errorf("expected Software to be %s, but was %s", testCase.metadata.Software, decoder.Metadata.Software)
				}

				if testCase.metadata.Source != decoder.Metadata.Source {
					t.Errorf("expected Source to be %s, but was %s", testCase.metadata.Source, decoder.Metadata.Source)
				}

				if testCase.metadata.Location != decoder.Metadata.Location {
					t.Errorf("expected Location to be %s, but was %s", testCase.metadata.Location, decoder.Metadata.Location)
				}

				if testCase.metadata.TrackNbr != decoder.Metadata.TrackNbr {
					t.Errorf("expected TrackNbr to be %s, but was %s", testCase.metadata.TrackNbr, decoder.Metadata.TrackNbr)
				}
			}

			newFile.Close()

			if err != nil {
				t.Fatal(err)
			}

			defer func() {
				err := os.Remove(newFile.Name())
				if err != nil {
					panic(err)
				}
			}()

			if nBuf.Format.SampleRate != buf.Format.SampleRate {
				t.Fatalf("sample rate didn't support roundtripping exp: %d, got: %d", buf.Format.SampleRate, nBuf.Format.SampleRate)
			}

			if nBuf.Format.NumChannels != buf.Format.NumChannels {
				t.Fatalf("the number of channels didn't support roundtripping exp: %d, got: %d", buf.Format.NumChannels, nBuf.Format.NumChannels)
			}

			if len(nBuf.Data) != len(buf.Data) {
				t.Fatalf("the reported number of frames didn't support roundtripping, exp: %d, got: %d", len(buf.Data), len(nBuf.Data))
			}

			for i := range len(buf.Data) {
				if !float32ApproxEqual(buf.Data[i], nBuf.Data[i], 1e-5) {
					end := min(i+3, len(buf.Data))

					t.Fatalf("frame value at position %d: %v\ndidn't match new buffer position %d: %v", i, buf.Data[i:end], i, nBuf.Data[i:end])
				}
			}
		})
	}
}

func TestEncoder_WriteFrame_PCM(t *testing.T) {
	os.Mkdir("testOutput", 0o777)

	tests := []struct {
		name     string
		bitDepth int
		format   int
		value    float32
	}{
		{"8bit PCM", 8, wavFormatPCM, 0.5},
		{"16bit PCM", 16, wavFormatPCM, 0.5},
		{"24bit PCM", 24, wavFormatPCM, -0.25},
		{"32bit PCM", 32, wavFormatPCM, 0.75},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outPath := path.Join("testOutput", "writeframe_"+tt.name+".wav")

			f, err := os.Create(outPath)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(outPath)

			enc := NewEncoder(f, 44100, tt.bitDepth, 1, tt.format)
			for range 100 {
				err := enc.WriteFrame(tt.value)
				if err != nil {
					t.Fatalf("WriteFrame failed: %v", err)
				}
			}

			if err := enc.Close(); err != nil {
				t.Fatal(err)
			}

			f.Close()

			verify, err := os.Open(outPath)
			if err != nil {
				t.Fatal(err)
			}
			defer verify.Close()

			dec := NewDecoder(verify)
			if !dec.IsValidFile() {
				t.Fatal("output should be a valid wav file")
			}
		})
	}
}

func TestEncoder_WriteFrame_Float(t *testing.T) {
	os.Mkdir("testOutput", 0o777)

	tests := []struct {
		name     string
		bitDepth int
		value    any
	}{
		{"float32 32bit", 32, float32(0.5)},
		{"float32 64bit", 64, float32(-0.25)},
		{"float64 32bit", 32, float64(0.75)},
		{"float64 64bit", 64, float64(-0.5)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outPath := path.Join("testOutput", "writeframe_float_"+tt.name+".wav")

			f, err := os.Create(outPath)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(outPath)

			enc := NewEncoder(f, 44100, tt.bitDepth, 1, wavFormatIEEEFloat)
			for range 100 {
				err := enc.WriteFrame(tt.value)
				if err != nil {
					t.Fatalf("WriteFrame failed: %v", err)
				}
			}

			if err := enc.Close(); err != nil {
				t.Fatal(err)
			}

			f.Close()

			verify, err := os.Open(outPath)
			if err != nil {
				t.Fatal(err)
			}
			defer verify.Close()

			dec := NewDecoder(verify)
			if !dec.IsValidFile() {
				t.Fatal("output should be a valid wav file")
			}
		})
	}
}

func TestEncoder_WriteFrame_G711(t *testing.T) {
	os.Mkdir("testOutput", 0o777)

	tests := []struct {
		name   string
		format int
	}{
		{"alaw", wavFormatALaw},
		{"mulaw", wavFormatMuLaw},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outPath := path.Join("testOutput", "writeframe_"+tt.name+".wav")

			f, err := os.Create(outPath)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(outPath)

			enc := NewEncoder(f, 8000, 8, 1, tt.format)
			for range 100 {
				err := enc.WriteFrame(float32(0.3))
				if err != nil {
					t.Fatalf("WriteFrame failed: %v", err)
				}
			}

			if err := enc.Close(); err != nil {
				t.Fatal(err)
			}

			f.Close()

			verify, err := os.Open(outPath)
			if err != nil {
				t.Fatal(err)
			}
			defer verify.Close()

			dec := NewDecoder(verify)
			if !dec.IsValidFile() {
				t.Fatal("output should be a valid wav file")
			}
		})
	}
}

func TestEncoder_WriteFrame_DefaultType(t *testing.T) {
	os.Mkdir("testOutput", 0o777)

	outPath := path.Join("testOutput", "writeframe_int16.wav")

	f, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(outPath)

	enc := NewEncoder(f, 44100, 16, 1, wavFormatPCM)
	// WriteFrame with int16 goes through the default case
	if err := enc.WriteFrame(int16(1000)); err != nil {
		t.Fatalf("WriteFrame with int16 failed: %v", err)
	}

	if err := enc.Close(); err != nil {
		t.Fatal(err)
	}

	f.Close()
}

func TestEncoder_Close_NilEncoder(t *testing.T) {
	var e *Encoder

	err := e.Close()
	if err != nil {
		t.Fatalf("Close on nil encoder should return nil, got %v", err)
	}
}

func TestEncoder_Close_NilWriter(t *testing.T) {
	e := &Encoder{}

	err := e.Close()
	if err != nil {
		t.Fatalf("Close with nil writer should return nil, got %v", err)
	}
}

func TestEncoder_AddBuffer_Nil(t *testing.T) {
	var buf bytes.Buffer

	e := NewEncoder(nopWriteSeeker{&buf}, 44100, 16, 1, wavFormatPCM)

	err := e.addBuffer(nil)
	if err == nil {
		t.Fatal("addBuffer(nil) should return error")
	}
}

func TestEncoder_Write_MultipleBuffers(t *testing.T) {
	os.Mkdir("testOutput", 0o777)

	outPath := path.Join("testOutput", "multi_write.wav")

	f, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(outPath)

	enc := NewEncoder(f, 44100, 16, 1, wavFormatPCM)
	format := &audio.Format{NumChannels: 1, SampleRate: 44100}

	// Write two separate buffers
	for range 2 {
		buf := &audio.Float32Buffer{
			Data:           []float32{0.1, 0.2, 0.3, -0.1, -0.2, -0.3},
			Format:         format,
			SourceBitDepth: 16,
		}

		err := enc.Write(buf)
		if err != nil {
			t.Fatal(err)
		}
	}

	if err := enc.Close(); err != nil {
		t.Fatal(err)
	}

	f.Close()

	verify, err := os.Open(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer verify.Close()

	dec := NewDecoder(verify)

	pcm, err := dec.FullPCMBuffer()
	if err != nil {
		t.Fatal(err)
	}

	if len(pcm.Data) != 12 {
		t.Fatalf("expected 12 samples, got %d", len(pcm.Data))
	}
}

// nopWriteSeeker wraps a bytes.Buffer to satisfy io.WriteSeeker.
type nopWriteSeeker struct {
	buf *bytes.Buffer
}

func (n nopWriteSeeker) Write(p []byte) (int, error) {
	written, err := n.buf.Write(p)
	if err != nil {
		return written, fmt.Errorf("buffer write failed: %w", err)
	}

	return written, nil
}

func (n nopWriteSeeker) Seek(offset int64, whence int) (int64, error) {
	return 0, nil
}
