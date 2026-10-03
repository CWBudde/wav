package wav

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"testing"

	"github.com/go-audio/audio"
)

type intMemoryWriter struct {
	data     []byte
	position int
	fail     bool
	short    bool
}

func (writer *intMemoryWriter) Write(data []byte) (int, error) {
	if writer.fail {
		return 0, io.ErrClosedPipe
	}

	count := len(data)
	if writer.short && count > 0 {
		count--
	}

	end := writer.position + count
	if end > len(writer.data) {
		writer.data = append(writer.data, make([]byte, end-len(writer.data))...)
	}

	copy(writer.data[writer.position:end], data[:count])
	writer.position = end

	return count, nil
}

func (writer *intMemoryWriter) Seek(offset int64, whence int) (int64, error) {
	position := int(offset)

	switch whence {
	case io.SeekCurrent:
		position += writer.position
	case io.SeekEnd:
		position += len(writer.data)
	}

	if position < 0 {
		return 0, io.ErrUnexpectedEOF
	}

	writer.position = position

	return int64(position), nil
}

func encodeIntTest(t *testing.T, bits, channels int, input []int, chunks []int) []byte {
	t.Helper()

	writer := new(intMemoryWriter)
	encoder := NewEncoder(writer, 48000, bits, channels, wavFormatPCM)
	encoder.UnknownChunks = []RawChunk{
		{ID: [4]byte{'J', 'U', 'N', 'K'}, Data: []byte{1, 2, 3}, BeforeData: true},
		{ID: [4]byte{'t', 'e', 's', 't'}, Data: []byte{4, 5, 6}, BeforeData: false},
	}

	before := append([]int(nil), input...)

	buffer := &audio.IntBuffer{
		Format:         &audio.Format{SampleRate: 48000, NumChannels: channels},
		SourceBitDepth: bits, Data: nil,
	}
	for start, iteration := 0, 0; start < len(input) || iteration == 0; iteration++ {
		count := min(len(input)-start, chunks[iteration%len(chunks)]*channels)

		buffer.Data = input[start : start+count]
		if err := encoder.WriteInt(buffer); err != nil {
			t.Fatal(err)
		}

		start += count
	}

	if !reflect.DeepEqual(input, before) && len(input) != 0 {
		t.Fatal("input modified")
	}

	if encoder.frames != len(input)/channels {
		t.Fatalf("frames%d want%d", encoder.frames, len(input)/channels)
	}

	if err := encoder.Close(); err != nil {
		t.Fatal(err)
	}

	return writer.data
}

//nolint:cyclop,lll // Independent byte and header goldens are checked together.
func TestEncoderWriteIntIndependentBytes(t *testing.T) {
	for _, test := range []struct {
		bits   int
		input  []int
		golden []byte
	}{
		{8, []int{-128, -1, 0, 1, 127}, []byte{0, 127, 128, 129, 255}},
		{16, []int{-32768, -1, 0, 1, 32767}, []byte{0, 128, 255, 255, 0, 0, 1, 0, 255, 127}},
		{24, []int{-8388608, -1, 0, 1, 8388607}, []byte{0, 0, 128, 255, 255, 255, 0, 0, 0, 1, 0, 0, 255, 255, 127}},
		{32, []int{-2147483648, -1, 0, 1, 2147483647}, []byte{0, 0, 0, 128, 255, 255, 255, 255, 0, 0, 0, 0, 1, 0, 0, 0, 255, 255, 255, 127}},
	} {
		t.Run(strconv.Itoa(test.bits), func(t *testing.T) {
			whole := encodeIntTest(t, test.bits, 1, test.input, []int{len(test.input)})

			chunked := encodeIntTest(t, test.bits, 1, test.input, []int{1, 3, 2})
			if !bytes.Equal(whole, chunked) {
				t.Fatal("chunk boundaries changed WAV")
			}

			chunks := regressionWAVChunks(t, whole)
			if !bytes.Equal(chunks["data"], test.golden) {
				t.Fatalf("PCM bytes%x want%x", chunks["data"], test.golden)
			}

			format := chunks["fmt "]
			if binary.LittleEndian.Uint16(format[0:2]) != 1 || binary.LittleEndian.Uint16(format[2:4]) != 1 || binary.LittleEndian.Uint32(format[4:8]) != 48000 || binary.LittleEndian.Uint32(format[8:12]) != 48000*uint32(test.bits/8) || binary.LittleEndian.Uint16(format[12:14]) != uint16(test.bits/8) || binary.LittleEndian.Uint16(format[14:16]) != uint16(test.bits) {
				t.Fatal("invalid PCM format geometry")
			}

			if !bytes.Equal(chunks["JUNK"], []byte{1, 2, 3}) || !bytes.Equal(chunks["test"], []byte{4, 5, 6}) {
				t.Fatal("raw metadata changed")
			}
		})
	}
}

//nolint:cyclop,lll // Exercise streaming interactions and exact PCM32 low bits.
func TestEncoderWriteIntStereoEmptyAndMixedWrites(t *testing.T) {
	input := []int{-2147483648, 2147483647, 16777217, -16777217, 1, -1}
	output := encodeIntTest(t, 32, 2, input, []int{1, 2})

	data := regressionWAVChunks(t, output)["data"]
	for i, value := range input {
		if int64(int32(binary.LittleEndian.Uint32(data[i*4:]))) != int64(value) {
			t.Fatalf("PCM32 precision sample%d", i)
		}
	}

	for _, bits := range []int{8, 16, 24, 32} {
		if len(regressionWAVChunks(t, encodeIntTest(t, bits, 2, nil, []int{1}))["data"]) != 0 {
			t.Fatal("empty data not empty")
		}
	}

	writer := new(intMemoryWriter)
	encoder := NewEncoder(writer, 48000, 16, 1, wavFormatPCM)

	format := &audio.Format{SampleRate: 48000, NumChannels: 1}
	if err := encoder.WriteInt(&audio.IntBuffer{Format: format, SourceBitDepth: 0, Data: []int{1, -1}}); err != nil {
		t.Fatal(err)
	}

	if err := encoder.Write(&audio.Float32Buffer{Format: format, SourceBitDepth: 0, Data: []float32{.5, -.5}}); err != nil {
		t.Fatal(err)
	}

	if err := encoder.WriteInt(&audio.IntBuffer{Format: format, SourceBitDepth: 0, Data: []int{32767, -32768}}); err != nil {
		t.Fatal(err)
	}

	if err := encoder.Close(); err != nil {
		t.Fatal(err)
	}

	if got := regressionWAVChunks(t, writer.data)["data"]; !bytes.Equal(got, []byte{1, 0, 255, 255, 0, 64, 0, 192, 255, 127, 0, 128}) {
		t.Fatalf("mixed writes%x", got)
	}

	before := append([]byte(nil), writer.data...)

	if err := encoder.WriteInt(&audio.IntBuffer{Format: format, SourceBitDepth: 0, Data: []int{1}}); err == nil {
		t.Fatal("write after Close accepted")
	}

	if !bytes.Equal(before, writer.data) {
		t.Fatal("closed write modified file")
	}
}

//nolint:cyclop,lll // Table-driven state and output rejection checks.
func TestEncoderWriteIntAtomicValidation(t *testing.T) {
	valid := func() *audio.IntBuffer {
		return &audio.IntBuffer{Format: &audio.Format{SampleRate: 48000, NumChannels: 2}, Data: []int{1, -1}, SourceBitDepth: 16}
	}

	for _, test := range []struct {
		name   string
		mutate func(*Encoder, *audio.IntBuffer)
	}{
		{"float", func(encoder *Encoder, _ *audio.IntBuffer) { encoder.WavAudioFormat = wavFormatIEEEFloat }},
		{"depth", func(encoder *Encoder, _ *audio.IntBuffer) { encoder.BitDepth = 12 }},
		{"nil format", func(_ *Encoder, b *audio.IntBuffer) { b.Format = nil }},
		{"channel mismatch", func(_ *Encoder, b *audio.IntBuffer) { b.Format.NumChannels = 1 }},
		{"rate mismatch", func(_ *Encoder, b *audio.IntBuffer) { b.Format.SampleRate = 44100 }},
		{"partial frame", func(_ *Encoder, b *audio.IntBuffer) { b.Data = []int{1} }},
		{"positive range", func(_ *Encoder, b *audio.IntBuffer) { b.Data = []int{1, 32768} }},
		{"negative range", func(_ *Encoder, b *audio.IntBuffer) { b.Data = []int{-32769, 0} }},
		{"source depth", func(_ *Encoder, b *audio.IntBuffer) { b.SourceBitDepth = 24 }},
		{"zero channels", func(encoder *Encoder, _ *audio.IntBuffer) { encoder.NumChans = 0 }},
		{"zero rate", func(encoder *Encoder, _ *audio.IntBuffer) { encoder.SampleRate = 0 }},
		{"large rate", func(encoder *Encoder, _ *audio.IntBuffer) { encoder.SampleRate = math.MaxInt32 }},
		{"alignment", func(encoder *Encoder, _ *audio.IntBuffer) { encoder.NumChans = math.MaxUint16 }},
		{"RIFF count", func(encoder *Encoder, _ *audio.IntBuffer) {
			encoder.WrittenBytes = math.MaxInt32
			encoder.frames = int(^uint(0) >> 1)
		}},
		{"negative count", func(encoder *Encoder, _ *audio.IntBuffer) { encoder.WrittenBytes = -1 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, alreadyStarted := range []bool{false, true} {
				writer := new(intMemoryWriter)
				encoder := NewEncoder(writer, 48000, 16, 2, wavFormatPCM)

				if alreadyStarted {
					if err := encoder.WriteInt(valid()); err != nil {
						t.Fatal(err)
					}
				}

				buffer := valid()
				test.mutate(encoder, buffer)
				state := *encoder

				before := append([]byte(nil), writer.data...)

				if err := encoder.WriteInt(buffer); err == nil {
					t.Fatal("invalid buffer accepted")
				}

				if !reflect.DeepEqual(*encoder, state) || !bytes.Equal(before, writer.data) {
					t.Fatal("rejection modified file or encoder")
				}
			}
		})
	}

	var nilEncoder *Encoder
	if err := nilEncoder.WriteInt(valid()); err == nil {
		t.Fatal("nil encoder accepted")
	}

	if err := NewEncoder(nil, 48000, 16, 2, wavFormatPCM).WriteInt(valid()); err == nil {
		t.Fatal("nil writer accepted")
	}

	if err := NewEncoder(new(intMemoryWriter), 48000, 16, 2, wavFormatPCM).WriteInt(nil); err == nil {
		t.Fatal("nil buffer accepted")
	}
}

func TestEncoderWriteIntIOFailures(t *testing.T) {
	for _, short := range []bool{false, true} {
		writer := new(intMemoryWriter)
		encoder := NewEncoder(writer, 48000, 16, 1, wavFormatPCM)

		buffer := &audio.IntBuffer{
			Format: &audio.Format{SampleRate: 48000, NumChannels: 1}, SourceBitDepth: 0, Data: []int{1},
		}
		if err := encoder.WriteInt(buffer); err != nil {
			t.Fatal(err)
		}

		writer.short = short
		writer.fail = !short

		want := io.ErrClosedPipe
		if short {
			want = io.ErrShortWrite
		}

		if err := encoder.WriteInt(buffer); !errors.Is(err, want) {
			t.Fatalf("I/O error%v want%v", err, want)
		}
	}
}

func TestEncoderWriteIntCueMetadata(t *testing.T) {
	writer := new(intMemoryWriter)
	encoder := NewEncoder(writer, 48000, 8, 1, wavFormatPCM)
	encoder.Metadata = &Metadata{CuePoints: []*CuePoint{{
		ID: [4]byte{7}, Position: 0, DataChunkID: [4]byte{'d', 'a', 't', 'a'}, ChunkStart: 0, BlockStart: 0, SampleOffset: 1,
	}}}

	buffer := &audio.IntBuffer{
		Format: &audio.Format{SampleRate: 48000, NumChannels: 1}, SourceBitDepth: 8, Data: []int{-128, 0, 127},
	}
	if err := encoder.WriteInt(buffer); err != nil {
		t.Fatal(err)
	}

	if err := encoder.Close(); err != nil {
		t.Fatal(err)
	}

	chunks := regressionWAVChunks(t, writer.data)
	if !bytes.Equal(chunks["data"], []byte{0, 128, 255}) {
		t.Fatal("odd PCM8 bytes changed")
	}

	wantCue := []byte{1, 0, 0, 0, 7, 0, 0, 0, 0, 0, 0, 0, 'd', 'a', 't', 'a', 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0}
	if !bytes.Equal(chunks["cue "], wantCue) {
		t.Fatalf("cue bytes%x want%x", chunks["cue "], wantCue)
	}
}

type intDiscardWriter struct{}

func (intDiscardWriter) Write(data []byte) (int, error)     { return len(data), nil }
func (intDiscardWriter) Seek(_ int64, _ int) (int64, error) { return 0, nil }

func TestEncoderWriteIntReusableAllocations(t *testing.T) {
	encoder := NewEncoder(intDiscardWriter{}, 48000, 32, 2, wavFormatPCM)

	buffer := &audio.IntBuffer{
		Format: &audio.Format{SampleRate: 48000, NumChannels: 2}, SourceBitDepth: 32, Data: make([]int, 2048),
	}
	if err := encoder.WriteInt(buffer); err != nil {
		t.Fatal(err)
	}

	if got := testing.AllocsPerRun(100, func() {
		if err := encoder.WriteInt(buffer); err != nil {
			t.Fatal(err)
		}
	}); got != 0 {
		t.Fatalf("reusable WriteInt allocated%g", got)
	}
}

func BenchmarkEncoderWriteIntPCM32(b *testing.B) {
	encoder := NewEncoder(intDiscardWriter{}, 48000, 32, 2, wavFormatPCM)

	buffer := &audio.IntBuffer{
		Format: &audio.Format{SampleRate: 48000, NumChannels: 2}, SourceBitDepth: 32, Data: make([]int, 2048),
	}
	if err := encoder.WriteInt(buffer); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.SetBytes(8192)
	b.ResetTimer()

	for range b.N {
		// Model a reusable bounded sink; avoid the RIFF total-size ceiling
		// limiting benchmark calibration after several gigabytes of writes.
		encoder.WrittenBytes = 44

		encoder.frames = 0
		if err := encoder.WriteInt(buffer); err != nil {
			b.Fatal(err)
		}
	}
}

//nolint:lll // Complete buffer geometry is explicit in the runnable example.
func ExampleEncoder_WriteInt() {
	writer := new(intMemoryWriter)

	encoder := NewEncoder(writer, 48000, 32, 1, wavFormatPCM)
	if err := encoder.WriteInt(&audio.IntBuffer{Format: &audio.Format{SampleRate: 48000, NumChannels: 1}, SourceBitDepth: 32, Data: []int{16777217, -16777217}}); err != nil {
		panic(err)
	}

	if err := encoder.Close(); err != nil {
		panic(err)
	}

	fmt.Printf("%x\n", writer.data[44:52])
	// Output: 01000001fffffffe
}
