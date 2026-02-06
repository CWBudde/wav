package wav

// RawChunk stores a non-core RIFF/WAV chunk for round-trip preservation.
type RawChunk struct {
	ID [4]byte
	// Size mirrors len(Data) for preserved chunks.
	Size uint32
	Data []byte
	// Order is the original chunk order index encountered during decode.
	Order int
	// BeforeData indicates if this chunk appeared before the data chunk.
	BeforeData bool
}

func (c RawChunk) Clone() RawChunk {
	out := c
	out.Data = append([]byte(nil), c.Data...)

	return out
}

func cloneRawChunks(chunks []RawChunk) []RawChunk {
	if len(chunks) == 0 {
		return nil
	}

	out := make([]RawChunk, len(chunks))
	for i := range chunks {
		out[i] = chunks[i].Clone()
	}

	return out
}
