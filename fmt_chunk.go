package wav

import "encoding/binary"

const (
	ksSubFormatGUIDTail0  = 0x00
	ksSubFormatGUIDTail1  = 0x00
	ksSubFormatGUIDTail2  = 0x10
	ksSubFormatGUIDTail3  = 0x00
	ksSubFormatGUIDTail4  = 0x80
	ksSubFormatGUIDTail5  = 0x00
	ksSubFormatGUIDTail6  = 0x00
	ksSubFormatGUIDTail7  = 0xAA
	ksSubFormatGUIDTail8  = 0x00
	ksSubFormatGUIDTail9  = 0x38
	ksSubFormatGUIDTail10 = 0x9B
	ksSubFormatGUIDTail11 = 0x71
)

// FmtChunk stores the parsed WAV fmt chunk, including extensible metadata.
type FmtChunk struct {
	FormatTag      uint16
	NumChannels    uint16
	SampleRate     uint32
	AvgBytesPerSec uint32
	BlockAlign     uint16
	BitsPerSample  uint16
	ExtraData      []byte
	Extensible     *FmtExtensible
}

// FmtExtensible stores WAVE_FORMAT_EXTENSIBLE extra fields.
type FmtExtensible struct {
	ValidBitsPerSample uint16
	ChannelMask        uint32
	SubFormat          [16]byte
	ExtraData          []byte
}

func (f *FmtChunk) Clone() *FmtChunk {
	if f == nil {
		return nil
	}

	out := *f

	out.ExtraData = append([]byte(nil), f.ExtraData...)
	if f.Extensible != nil {
		ext := *f.Extensible
		ext.ExtraData = append([]byte(nil), f.Extensible.ExtraData...)
		out.Extensible = &ext
	}

	return &out
}

func (f *FmtChunk) EffectiveFormatTag() uint16 {
	if f == nil {
		return 0
	}

	if f.FormatTag == wavFormatExtensible && f.Extensible != nil {
		return binary.LittleEndian.Uint16(f.Extensible.SubFormat[:2])
	}

	return f.FormatTag
}

func makeSubFormatGUID(formatTag uint16) [16]byte {
	var guid [16]byte
	binary.LittleEndian.PutUint32(guid[:4], uint32(formatTag))
	guid[4] = ksSubFormatGUIDTail0
	guid[5] = ksSubFormatGUIDTail1
	guid[6] = ksSubFormatGUIDTail2
	guid[7] = ksSubFormatGUIDTail3
	guid[8] = ksSubFormatGUIDTail4
	guid[9] = ksSubFormatGUIDTail5
	guid[10] = ksSubFormatGUIDTail6
	guid[11] = ksSubFormatGUIDTail7
	guid[12] = ksSubFormatGUIDTail8
	guid[13] = ksSubFormatGUIDTail9
	guid[14] = ksSubFormatGUIDTail10
	guid[15] = ksSubFormatGUIDTail11

	return guid
}
