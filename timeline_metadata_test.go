package wav

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/go-audio/audio"
	"github.com/go-audio/riff"
)

func timelineChunk(id [4]byte, payload []byte) *riff.Chunk {
	return &riff.Chunk{ID: id, Size: len(payload), R: bytes.NewReader(payload)}
}

type regressionWriteSeeker struct {
	data     []byte
	position int
}

func (writer *regressionWriteSeeker) Write(data []byte) (int, error) {
	end := writer.position + len(data)
	if end > len(writer.data) {
		writer.data = append(writer.data, make([]byte, end-len(writer.data))...)
	}

	copy(writer.data[writer.position:end], data)
	writer.position = end

	return len(data), nil
}

func (writer *regressionWriteSeeker) Seek(offset int64, whence int) (int64, error) {
	base := int64(0)

	switch whence {
	case io.SeekCurrent:
		base = int64(writer.position)
	case io.SeekEnd:
		base = int64(len(writer.data))
	case io.SeekStart:
	default:
		return 0, fmt.Errorf("%w: invalid seek", errInvalidMetadata)
	}

	position := base + offset
	if position < 0 || position > int64(len(writer.data)) {
		return 0, fmt.Errorf("%w: invalid position", errInvalidMetadata)
	}

	writer.position = int(position)

	return position, nil
}

func TestFLAssociatedDataFixture(t *testing.T) {
	file, err := os.Open("fixtures/flloop.wav")
	requireTimelineSuccess(t, err)

	defer file.Close()

	decoder := NewDecoder(file)
	decoder.ReadMetadata()
	requireTimelineSuccess(t, decoder.Err())

	if !reflect.DeepEqual(decoder.Metadata.AssociatedData, expectedFLAssociatedData()) {
		t.Fatal("FL Studio labels/regions not decoded exactly")
	}
}

func expectedFLAssociatedData() *AssociatedData {
	const hat = "Hat"

	labels := []string{
		"Hat + Kick",
		hat,
		hat,
		hat,
		"Snare + Clap + Hat",
		hat,
		hat,
		hat,
		"Kick + Hat",
		hat,
		hat,
		hat,
		"Clap + Snare + Hat",
		hat,
		"Kick + Hat",
		hat,
	}

	a := new(AssociatedData)
	for i, label := range labels {
		a.Labels = append(a.Labels, CueLabel{CuePointID: uint32(i + 1), Text: label})
		a.Regions = append(a.Regions, CueRegion{
			CuePointID: uint32(i + 1), SampleLength: 0x1a5e, PurposeID: [4]byte{'b', 'e', 'a', 't'},
			Country: 0, Language: 0, Dialect: 0, CodePage: 0, Text: "",
		})
	}

	return a
}

func TestTimelineStandaloneChunkRoundTrip(t *testing.T) {
	cues := []*CuePoint{
		{
			ID: [4]byte{},
			DataChunkID: [4]byte{
				'd',
				'a',
				't',
				'a',
			},
			SampleOffset: 3,
		},
		{
			ID: [4]byte{
				255,
				255,
				255,
				255,
			},
			Position: math.MaxUint32,
			DataChunkID: [4]byte{
				'd',
				'a',
				't',
				'a',
			},
			SampleOffset: math.MaxUint32,
		},
	}
	a := &AssociatedData{
		Labels: []CueLabel{{0, "é"}, {math.MaxUint32, string(rune(0x6a4b))}},
		Notes:  []CueLabel{{0, "odd!"}},
		Regions: []CueRegion{{
			CuePointID: math.MaxUint32, SampleLength: math.MaxUint32, PurposeID: [4]byte{'r', 'g', 'n', ' '},
			Country: 1, Language: 2, Dialect: 3, CodePage: 65001, Text: "régiøn",
		}},
		UnknownSubchunks: []RawChunk{{ID: [4]byte{'x', 'x', 'x', 'x'}, Size: 3, Data: []byte{1, 2, 3}}},
	}
	cue, err := EncodeCueChunk(cues)
	requireTimelineSuccess(t, err)
	adtl, err := EncodeAssociatedDataChunk(a)
	requireTimelineSuccess(t, err)

	if cue.Size != uint32(len(cue.Data)) || adtl.Size != uint32(len(adtl.Data)) {
		t.Fatal("serializer size mismatch")
	}

	decoder := NewDecoder(bytes.NewReader(nil))
	// Cue references must survive reversed chunk order.
	requireTimelineSuccess(t, DecodeAssociatedDataChunk(decoder, timelineChunk(CIDList, adtl.Data)))
	requireTimelineSuccess(t, DecodeCueChunk(decoder, timelineChunk(CIDCue, cue.Data)))

	if !reflect.DeepEqual(decoder.Metadata.CuePoints, cues) || !reflect.DeepEqual(decoder.Metadata.AssociatedData, a) {
		t.Fatalf("roundtrip mismatch %+v %+v", decoder.Metadata.CuePoints, decoder.Metadata.AssociatedData)
	}

	other := labeledAssociatedData(CueLabel{17, "second"})
	chunk, err := EncodeAssociatedDataChunk(other)
	requireTimelineSuccess(t, err)
	requireTimelineSuccess(t, DecodeAssociatedDataChunk(decoder, timelineChunk(CIDList, chunk.Data)))

	if len(decoder.Metadata.AssociatedData.Labels) != 3 || decoder.Metadata.AssociatedData.Labels[2] != other.Labels[0] {
		t.Fatal("additional adtl list overwritten")
	}
}

func TestTimelineMalformedPayloadsAreAtomic(t *testing.T) {
	const keptLabel = "keep"

	good, err := EncodeAssociatedDataChunk(labeledAssociatedData(CueLabel{1, keptLabel}))
	requireTimelineSuccess(t, err)

	for _, bad := range [][]byte{
		nil,
		[]byte("adt"),
		append([]byte("adtl"),
			make([]byte,
				7)...),
		append([]byte("adtllabl"),
			255,
			255,
			255,
			255),
		append([]byte("adtllabl"),
			3,
			0,
			0,
			0,
			1,
			2,
			3,
			0),
		append(append([]byte("adtlltxt"),
			19,
			0,
			0,
			0),
			make([]byte,
				20)...),
		append([]byte("adtllabl"),
			5,
			0,
			0,
			0,
			1,
			0,
			0,
			0,
			0),
		append(append([]byte(nil),
			good.Data...),
			[]byte("labl\xff\xff\xff\xff")...),
	} {
		decoder := NewDecoder(bytes.NewReader(nil))
		requireTimelineSuccess(t, DecodeAssociatedDataChunk(decoder, timelineChunk(CIDList, good.Data)))

		before := *decoder.Metadata.AssociatedData
		if err := DecodeAssociatedDataChunk(decoder, timelineChunk(CIDList, bad)); err == nil {
			t.Fatalf("accepted malformed adtl %x", bad)
		}

		if !reflect.DeepEqual(before, *decoder.Metadata.AssociatedData) {
			t.Fatal("malformed list published partial data")
		}
	}
}

func TestMalformedCuePayloadsAreAtomic(t *testing.T) {
	for _, bad := range [][]byte{
		nil,
		{1},
		{
			1,
			0,
			0,
			0,
		},
		{
			255,
			255,
			255,
			255,
		},
		append([]byte{
			0,
			0,
			0,
			0,
		},
			make([]byte,
				24)...),
	} {
		decoder := NewDecoder(bytes.NewReader(nil))
		keep := []*CuePoint{{SampleOffset: 7}}

		decoder.Metadata = &Metadata{CuePoints: keep}
		if err := DecodeCueChunk(decoder, timelineChunk(CIDCue, bad)); err == nil {
			t.Fatalf("accepted malformed cue %x", bad)
		}

		if !reflect.DeepEqual(decoder.Metadata.CuePoints, keep) {
			t.Fatal("malformed cue published partial rows")
		}
	}
}

func TestTimelineInvalidArguments(t *testing.T) {
	for _, decode := range []func(*Decoder,
		*riff.Chunk) error{
		DecodeCueChunk,
		DecodeAssociatedDataChunk,
		DecodeListChunk,
	} {
		if decode(nil, timelineChunk(CIDCue, nil)) == nil || decode(NewDecoder(bytes.NewReader(nil)), nil) == nil {
			t.Fatal("nil decoder/chunk accepted")
		}
	}

	if _, err := EncodeCueChunk([]*CuePoint{nil}); err == nil {
		t.Fatal("nil cue row accepted")
	}

	if _, err := EncodeAssociatedDataChunk(nil); err == nil {
		t.Fatal("nil associated data accepted")
	}

	if _, err := EncodeAssociatedDataChunk(labeledAssociatedData(CueLabel{1, "nul\x00text"})); err == nil {
		t.Fatal("embedded NUL accepted")
	}

	if _,
		err := EncodeAssociatedDataChunk(labeledAssociatedData(CueLabel{
		1,
		strings.Repeat("x",
			MaxMetadataChunkBytes),
	})); err == nil {
		t.Fatal("oversized label accepted")
	}
}

func TestEmptyCueAndOversizedPayload(t *testing.T) {
	zero, err := EncodeCueChunk(nil)
	requireTimelineSuccess(t, err)

	decoder := NewDecoder(bytes.NewReader(nil))
	if err := DecodeCueChunk(decoder,
		timelineChunk(CIDCue,
			zero.Data)); err != nil || len(decoder.Metadata.CuePoints) != 0 {
		t.Fatal("empty cue failed")
	}

	for _, id := range [][4]byte{CIDCue, CIDList} {
		chunk := &riff.Chunk{ID: id, Size: MaxMetadataChunkBytes + 1, R: bytes.NewReader(nil)}
		if _, err := metadataPayload(chunk); err == nil {
			t.Fatal("unbounded chunk allocation allowed")
		}
	}
}

func TestTimelineReadMetadataBeforeFormatAndAfterAudio(t *testing.T) {
	a := labeledAssociatedData(CueLabel{CuePointID: 1, Text: "before"})
	adtl, err := EncodeAssociatedDataChunk(a)
	requireTimelineSuccess(t, err)

	writer := new(regressionWriteSeeker)
	encoder := NewEncoder(writer, 48000, 8, 1, 1)
	encoder.Metadata = &Metadata{CuePoints: []*CuePoint{{
		ID: [4]byte{1},
		DataChunkID: [4]byte{
			'd',
			'a',
			't',
			'a',
		},
		SampleOffset: 1,
	}}}
	requireTimelineSuccess(t, encoder.Write(&audio.Float32Buffer{Data: []float32{0, 0, 0}}))
	requireTimelineSuccess(t, encoder.Close())
	// Insert a standard adtl list before fmt; the cue list is after odd data.
	chunkWriter := new(regressionWriteSeeker)
	chunkEncoder := NewEncoder(chunkWriter, 48000, 8, 1, 1)
	requireTimelineSuccess(t, chunkEncoder.writeRawChunk(adtl))

	file := append([]byte(nil), writer.data[:12]...)
	file = append(file, chunkWriter.data...)
	file = append(file, writer.data[12:]...)
	binary.LittleEndian.PutUint32(file[4:8], uint32(len(file)-8))
	decoder := NewDecoder(bytes.NewReader(file))
	decoder.ReadInfo()
	requireTimelineSuccess(t, decoder.Err())
	decoder.ReadMetadata()
	requireTimelineSuccess(t, decoder.Err())

	if !reflect.DeepEqual(decoder.Metadata.AssociatedData, a) || len(decoder.Metadata.CuePoints) != 1 {
		t.Fatal("header metadata suppressed post-audio cues or duplicated adtl")
	}

	requireTimelineSuccess(t, decoder.Rewind())

	if !reflect.DeepEqual(decoder.Metadata.AssociatedData, a) || len(decoder.Metadata.CuePoints) != 1 {
		t.Fatal("forwarding PCM lost cached cues or duplicated header adtl")
	}
}

func TestHeaderMetadataAggregateLimit(t *testing.T) {
	decoder := NewDecoder(bytes.NewReader(nil))
	decoder.metadataBytes = maxMetadataBytes - 3
	chunk := timelineChunk(CIDList, []byte("adtl"))

	var rewindBytes int64
	if err := decoder.processNonFmtChunk(chunk, &rewindBytes); err == nil {
		t.Fatal("header metadata aggregate limit was bypassed")
	}

	if decoder.Metadata != nil || decoder.metadataBytes != maxMetadataBytes-3 {
		t.Fatal("rejected header metadata changed decoder state")
	}
}

func TestReadMetadataTruncatedPayloadErrors(t *testing.T) {
	for _, id := range [][4]byte{CIDCue, {'x', 'x', 'x', 'x'}, CIDList} {
		writer := new(regressionWriteSeeker)
		initialEncoder := NewEncoder(writer, 48000, 8, 1, 1)
		requireTimelineSuccess(t, initialEncoder.Write(&audio.Float32Buffer{Data: []float32{0, 0}}))
		requireTimelineSuccess(t, initialEncoder.Close())

		writer.data = append(writer.data, id[:]...)
		writer.data = append(writer.data, 4, 0, 0, 0)
		binary.LittleEndian.PutUint32(writer.data[4:8], uint32(len(writer.data)-8))
		decoder := NewDecoder(bytes.NewReader(writer.data))
		decoder.ReadMetadata()

		if decoder.Err() == nil {
			t.Fatalf("truncated %q payload silently accepted", id)
		}
	}
}

func TestReadMetadataRejectsMissingFinalPadding(t *testing.T) {
	writer := new(regressionWriteSeeker)
	encoder := NewEncoder(writer, 48000, 8, 1, 1)
	encoder.SetRawChunks([]RawChunk{{ID: [4]byte{'o', 'd', 'd', ' '}, Data: []byte{7}}})
	requireTimelineSuccess(t, encoder.Write(&audio.Float32Buffer{Data: []float32{0, 0}}))
	requireTimelineSuccess(t, encoder.Close())

	file := writer.data[:len(writer.data)-1]
	binary.LittleEndian.PutUint32(file[4:8], uint32(len(file)-8))
	decoder := NewDecoder(bytes.NewReader(file))
	decoder.ReadMetadata()

	if !errors.Is(decoder.Err(), io.ErrUnexpectedEOF) || decoder.metadataRead {
		t.Fatal("missing final alignment byte was accepted as normal EOF")
	}
}

func TestInfoSubchunkBoundsAndPadding(t *testing.T) {
	const keptLabel = "keep"

	decoder := NewDecoder(bytes.NewReader(nil))
	decoder.Metadata = &Metadata{Title: keptLabel}

	bad := append([]byte("INFOINAM"), 255, 255, 255, 255)
	if err := DecodeListChunk(decoder, timelineChunk(CIDList, bad)); err == nil || decoder.Metadata.Title != keptLabel {
		t.Fatal("forged INFO length accepted/mutated title")
	}

	buffer := new(bytes.Buffer)
	buffer.WriteString("INFO")
	requireTimelineSuccess(t, appendListRecord(buffer, markerINAM, []byte("ab\x00")))
	requireTimelineSuccess(t, appendListRecord(buffer, markerIART, []byte("é\x00")))
	requireTimelineSuccess(t, DecodeListChunk(decoder, timelineChunk(CIDList, buffer.Bytes())))

	if decoder.Metadata.Title != "ab" || decoder.Metadata.Artist != "é" {
		t.Fatal("INFO per-record padding lost")
	}

	encoded, err := encodeInfoChunk(&Encoder{Metadata: &Metadata{Title: "ab", Artist: "é"}})
	requireTimelineSuccess(t, err)

	other := NewDecoder(bytes.NewReader(nil))
	requireTimelineSuccess(t, DecodeListChunk(other, timelineChunk(CIDList, encoded)))

	if other.Metadata.Title != "ab" || other.Metadata.Artist != "é" {
		t.Fatal("INFO encoder alignment mismatch")
	}
}

func TestUnknownChunkOddPayloadRoundTrip(t *testing.T) {
	writer := new(regressionWriteSeeker)
	initialEncoder := NewEncoder(writer, 48000, 8, 1, 1)
	initialEncoder.SetRawChunks([]RawChunk{
		{
			ID: [4]byte{
				'p',
				'r',
				'e',
				' ',
			},
			Data: []byte{
				1,
				2,
				3,
			},
			BeforeData: true,
		},
		{
			ID: [4]byte{
				'p',
				'o',
				's',
				't',
			},
			Data: []byte{
				4,
				5,
				6,
			},
			BeforeData: false,
		},
		{
			ID:         CIDList,
			Data:       []byte("zzzzodd"),
			BeforeData: false,
		},
	})
	requireTimelineSuccess(t, initialEncoder.Write(&audio.Float32Buffer{Data: []float32{0}}))
	requireTimelineSuccess(t, initialEncoder.Close())

	decoder := NewDecoder(bytes.NewReader(writer.data))
	decoder.ReadMetadata()
	requireTimelineSuccess(t, decoder.Err())

	chunks := decoder.RawChunks()
	if len(chunks) != 3 {
		t.Fatalf("unknown chunks %v", chunks)
	}

	for i, want := range [][]byte{{1, 2, 3}, {4, 5, 6}, []byte("zzzzodd")} {
		if !bytes.Equal(chunks[i].Data, want) || chunks[i].Size != uint32(len(want)) {
			t.Fatal("alignment included in preserved payload")
		}
	}

	if !chunks[0].BeforeData || chunks[1].BeforeData || chunks[2].BeforeData {
		t.Fatal("data placement lost")
	}

	second := new(regressionWriteSeeker)
	encoder := NewEncoder(second, 48000, 8, 1, 1)
	encoder.SetRawChunks(chunks)
	requireTimelineSuccess(t, encoder.Write(&audio.Float32Buffer{Data: []float32{0}}))
	requireTimelineSuccess(t, encoder.Close())

	if !bytes.Equal(writer.data, second.data) {
		t.Fatal("unknown chunk file bytes changed on roundtrip")
	}
}

func TestEncoderMetadataWritesCueAndAssociatedData(t *testing.T) {
	writer := new(regressionWriteSeeker)
	meta := &Metadata{
		CuePoints: []*CuePoint{{
			ID: [4]byte{1},
			DataChunkID: [4]byte{
				'd',
				'a',
				't',
				'a',
			},
			SampleOffset: 1,
		}},
		AssociatedData: &AssociatedData{
			Labels: []CueLabel{{1, "é"}}, Notes: nil, UnknownSubchunks: nil,
			Regions: []CueRegion{{
				CuePointID: 1, SampleLength: 2, PurposeID: [4]byte{},
				Country: 0, Language: 0, Dialect: 0, CodePage: 0, Text: "region",
			}},
		},
	}
	initialEncoder := NewEncoder(writer, 48000, 8, 1, 1)
	initialEncoder.Metadata = meta
	requireTimelineSuccess(t, initialEncoder.Write(&audio.Float32Buffer{Data: []float32{0, 0, 0}}))
	requireTimelineSuccess(t, initialEncoder.Close())

	if int(binary.LittleEndian.Uint32(writer.data[4:8])) != len(writer.data)-8 {
		t.Fatal("RIFF size excludes metadata")
	}

	decoder := NewDecoder(bytes.NewReader(writer.data))
	decoder.ReadMetadata()
	requireTimelineSuccess(t, decoder.Err())

	if !reflect.DeepEqual(decoder.Metadata.CuePoints,
		meta.CuePoints) || !reflect.DeepEqual(decoder.Metadata.AssociatedData,
		meta.AssociatedData) {
		t.Fatal("Encoder.Metadata dropped timeline")
	}
}

func requireTimelineSuccess(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}
}

func labeledAssociatedData(labels ...CueLabel) *AssociatedData {
	return &AssociatedData{Labels: labels, Notes: nil, Regions: nil, UnknownSubchunks: nil}
}
