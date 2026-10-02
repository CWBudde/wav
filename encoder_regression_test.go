package wav

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-audio/audio"
)

const (
	regressionWriteBuffer = "Write"
	regressionWriteFrame  = "WriteFrame"
	regressionOddTitle    = "after odd data"
)

func encodeRegressionWAV(t *testing.T, bitDepth, audioFormat int, write func(*Encoder)) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "encoded.wav")

	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { file.Close() })

	enc := NewEncoder(file, 48000, bitDepth, 1, audioFormat)
	write(enc)

	if err := enc.Close(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return data
}

// Check the on-disk chunk boundaries independently of the decoder, whose float
// output may clamp or narrow samples and hide encoder corruption.
func regressionWAVChunks(t *testing.T, data []byte) map[string][]byte {
	t.Helper()

	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		t.Fatal("invalid RIFF/WAVE header")
	}

	if got := binary.LittleEndian.Uint32(data[4:8]); int(got) != len(data)-8 {
		t.Fatalf("RIFF size = %d, want %d", got, len(data)-8)
	}

	chunks := make(map[string][]byte)

	for offset := 12; offset < len(data); {
		if offset+8 > len(data) {
			t.Fatalf("truncated chunk header at byte %d", offset)
		}

		id := string(data[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))

		end := offset + 8 + size
		if end > len(data) {
			t.Fatalf("chunk %q extends beyond file", id)
		}

		chunks[id] = data[offset+8 : end]
		if size%2 == 1 {
			checkRegressionPadding(t, data, id, end)
			end++
		}

		offset = end
	}

	return chunks
}

func checkRegressionPadding(t *testing.T, data []byte, id string, offset int) {
	t.Helper()

	if offset >= len(data) || data[offset] != 0 {
		t.Fatalf("chunk %q lacks zero alignment padding", id)
	}
}

func TestEncoderIEEEFloatPreservesBits(t *testing.T) {
	bits32 := []uint32{
		0, 0x80000000, 0x40000000, 0xc0600000, // signed zero and values beyond unity
		0x7f800000, 0xff800000, // infinities
		0x7fc12345, 0xffc54321, 0x7f812345, // quiet and signaling NaN payloads
		1, 0x807fffff, 0x7f7fffff, // subnormal and maximum finite values
	}
	bits64 := []uint64{
		0, 0x8000000000000000, 0x4000000000000000, 0xc00c000000000000,
		0x7ff0000000000000, 0xfff0000000000000,
		0x7ff8123456789abc, 0xfff8abcdef123456, 0x7ff0123456789abc,
		1, 0x800fffffffffffff, 0x7fefffffffffffff,
	}

	samples := make([]float32, len(bits32))
	for i, bits := range bits32 {
		samples[i] = math.Float32frombits(bits)
	}

	for _, method := range []string{regressionWriteBuffer, regressionWriteFrame} {
		for _, depth := range []int{32, 64} {
			t.Run(method+"/"+map[int]string{32: "float32", 64: "float64"}[depth], func(t *testing.T) {
				runRegressionFloatEncode(t, method, depth, samples, bits32, bits64)
			})
		}
	}
}

func runRegressionFloatEncode(
	t *testing.T, method string, depth int, samples []float32, bits32 []uint32, bits64 []uint64,
) {
	t.Helper()

	var want bytes.Buffer

	data := encodeRegressionWAV(t, depth, wavFormatIEEEFloat, func(enc *Encoder) {
		regressionWriteFloats(t, enc, method, samples, bits64)
	})
	switch {
	case depth == 32:
		regressionWriteBinary(t, &want, bits32)
	case method == regressionWriteFrame:
		regressionWriteBinary(t, &want, bits64)
	default:
		for _, sample := range samples {
			regressionWriteBinary(t, &want, math.Float64bits(float64(sample)))
		}
	}

	got := regressionWAVChunks(t, data)["data"]
	if !bytes.Equal(got, want.Bytes()) {
		t.Fatalf("encoded float bits = %x, want %x", got, want.Bytes())
	}
}

func regressionWriteFloats(t *testing.T, enc *Encoder, method string, samples []float32, bits64 []uint64) {
	t.Helper()

	switch {
	case method == regressionWriteBuffer:
		if err := enc.Write(&audio.Float32Buffer{
			Format: &audio.Format{SampleRate: 48000, NumChannels: 1}, Data: samples,
		}); err != nil {
			t.Fatal(err)
		}
	case enc.BitDepth == 32:
		for _, sample := range samples {
			if err := enc.WriteFrame(sample); err != nil {
				t.Fatal(err)
			}
		}
	default:
		for _, bits := range bits64 {
			if err := enc.WriteFrame(math.Float64frombits(bits)); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestEncoderOddDataPadding(t *testing.T) {
	for _, method := range []string{regressionWriteBuffer, regressionWriteFrame} {
		for _, postChunks := range []bool{false, true} {
			name := method + "/no-post-chunks"
			if postChunks {
				name = method + "/post-chunks"
			}

			t.Run(name, func(t *testing.T) {
				runRegressionOddEncode(t, method, postChunks)
			})
		}
	}
}

func runRegressionOddEncode(t *testing.T, method string, postChunks bool) {
	t.Helper()
	data := encodeRegressionWAV(t, 8, wavFormatPCM, func(enc *Encoder) {
		if postChunks {
			enc.UnknownChunks = []RawChunk{{ID: [4]byte{'J', 'U', 'N', 'K'}, Data: []byte{3, 4, 5}}}
			enc.Metadata = &Metadata{Title: regressionOddTitle}
		}

		if method == regressionWriteBuffer {
			if err := enc.Write(&audio.Float32Buffer{
				Format: &audio.Format{SampleRate: 48000, NumChannels: 1}, Data: []float32{-2, 0, 2},
			}); err != nil {
				t.Fatal(err)
			}
		} else {
			for _, sample := range []float64{-2, 0, 2} {
				if err := enc.WriteFrame(sample); err != nil {
					t.Fatal(err)
				}
			}
		}
	})

	chunks := regressionWAVChunks(t, data)
	if got := chunks["data"]; !bytes.Equal(got, []byte{0, 128, 255}) {
		t.Fatalf("PCM8 data = %v, want clipped samples without padding", got)
	}

	if postChunks {
		if !bytes.Equal(chunks["JUNK"], []byte{3, 4, 5}) {
			t.Fatalf("post-data chunk = %v", chunks["JUNK"])
		}

		if !bytes.Contains(chunks["LIST"], []byte(regressionOddTitle)) {
			t.Fatalf("post-data metadata = %q", chunks["LIST"])
		}
	}
}

func TestNewEncoderAllocatesBufferLazily(t *testing.T) {
	var output bytes.Buffer

	enc := NewEncoder(nopWriteSeeker{&output}, 48000, 32, 2, wavFormatIEEEFloat)
	if capacity := enc.buf.Cap(); capacity != 0 {
		t.Fatalf("new encoder preallocated %d bytes before receiving audio", capacity)
	}

	buf := &audio.Float32Buffer{
		Format: &audio.Format{SampleRate: 48000, NumChannels: 2}, Data: make([]float32, 64),
	}

	for range 64 {
		before := output.Len()

		if err := enc.Write(buf); err != nil {
			t.Fatal(err)
		}

		if output.Len()-before < len(buf.Data)*4 {
			t.Fatal("Write did not flush samples to its writer")
		}

		if enc.buf.Len() != 0 || enc.buf.Cap() > 4096 {
			t.Fatalf("small streaming writes retain excessive buffer: len=%d cap=%d", enc.buf.Len(), enc.buf.Cap())
		}
	}
}
