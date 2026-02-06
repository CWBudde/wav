package wav

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/go-audio/audio"
)

func TestChunkInventory_RoundTripUnknownFixture(t *testing.T) {
	input := makeWavWithUnknownChunks(t)

	before, err := parseWavChunks(input)
	if err != nil {
		t.Fatalf("parse input chunks: %v", err)
	}

	dec := NewDecoder(bytes.NewReader(input))
	dec.ReadMetadata()
	if err := dec.Err(); err != nil {
		t.Fatalf("read metadata: %v", err)
	}

	if err := dec.Rewind(); err != nil {
		t.Fatalf("rewind: %v", err)
	}

	pcm, err := dec.FullPCMBuffer()
	if err != nil {
		t.Fatalf("decode PCM: %v", err)
	}

	outPath := filepath.Join(t.TempDir(), "inventory_roundtrip.wav")
	out, err := os.Create(outPath)
	if err != nil {
		t.Fatalf("create output: %v", err)
	}

	enc := NewEncoderFromDecoder(out, dec)
	if err := enc.Write(pcm); err != nil {
		t.Fatalf("encode PCM: %v", err)
	}

	if err := enc.Close(); err != nil {
		t.Fatalf("close encoder: %v", err)
	}

	if err := out.Close(); err != nil {
		t.Fatalf("close output: %v", err)
	}

	after, err := parseWavChunksFromFile(outPath)
	if err != nil {
		t.Fatalf("parse output chunks: %v", err)
	}

	beforeInventory := buildChunkInventory(before)
	afterInventory := buildChunkInventory(after)
	if !reflect.DeepEqual(beforeInventory, afterInventory) {
		t.Fatalf("chunk inventory mismatch:\n before=%v\n after=%v", beforeInventory, afterInventory)
	}
}

func TestUnsupportedCompressedFormats_ErrorMessageIncludesCodec(t *testing.T) {
	testCases := []struct {
		path string
		name string
		tag  uint16
	}{
		{path: "fixtures/truspech.wav", name: "TrueSpeech", tag: 34},
		{path: "fixtures/voxware.wav", name: "Voxware", tag: 6172},
	}

	for _, tc := range testCases {
		t.Run(filepath.Base(tc.path), func(t *testing.T) {
			f, err := os.Open(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			d := NewDecoder(f)
			_, err = d.FullPCMBuffer()
			if !errors.Is(err, ErrUnsupportedCompressedFormat) {
				t.Fatalf("expected ErrUnsupportedCompressedFormat, got %v", err)
			}

			msg := err.Error()
			if !strings.Contains(msg, tc.name) {
				t.Fatalf("error %q should include codec name %q", msg, tc.name)
			}

			if !strings.Contains(msg, strconv.Itoa(int(tc.tag))) {
				t.Fatalf("error %q should include format tag %d", msg, tc.tag)
			}
		})
	}
}

func TestDecoder_StreamingParityAcrossSupportedFormats(t *testing.T) {
	testCases := []string{
		"fixtures/kick.wav",
		"fixtures/bass.wav",
		"fixtures/M1F1-float32-AFsp.wav",
		"fixtures/M1F1-float64-AFsp.wav",
		"fixtures/M1F1-Alaw-AFsp.wav",
		"fixtures/M1F1-mulaw-AFsp.wav",
		"fixtures/addf8-GSM-GW.wav",
	}

	for _, fixture := range testCases {
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			in, err := os.Open(fixture)
			if err != nil {
				t.Fatal(err)
			}

			fullDec := NewDecoder(in)
			fullBuf, err := fullDec.FullPCMBuffer()
			if err != nil {
				in.Close()
				t.Fatalf("full decode: %v", err)
			}

			if err := in.Close(); err != nil {
				t.Fatalf("close input: %v", err)
			}

			in2, err := os.Open(fixture)
			if err != nil {
				t.Fatal(err)
			}
			defer in2.Close()

			streamDec := NewDecoder(in2)
			streamDec.ReadInfo()
			if err := streamDec.Err(); err != nil {
				t.Fatalf("read info: %v", err)
			}

			streamed := make([]float32, 0, len(fullBuf.Data))
			tmp := &audio.Float32Buffer{
				Format: streamDec.Format(),
				Data:   make([]float32, 257),
			}

			for {
				n, err := streamDec.PCMBuffer(tmp)
				if err != nil {
					t.Fatalf("stream decode: %v", err)
				}

				if n == 0 {
					break
				}

				streamed = append(streamed, tmp.Data[:n]...)
			}

			if len(streamed) != len(fullBuf.Data) {
				t.Fatalf("sample count mismatch: stream=%d full=%d", len(streamed), len(fullBuf.Data))
			}

			for i := range fullBuf.Data {
				if !float32ApproxEqual(streamed[i], fullBuf.Data[i], 1e-6) {
					t.Fatalf("sample %d mismatch: stream=%f full=%f", i, streamed[i], fullBuf.Data[i])
				}
			}
		})
	}
}
