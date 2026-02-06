package wav

// FormatChunk returns a copy of the parsed fmt chunk, if available.
func (d *Decoder) FormatChunk() *FmtChunk {
	if d == nil || d.FmtChunk == nil {
		return nil
	}

	return d.FmtChunk.Clone()
}

// RawChunks returns a copy of preserved non-core chunks.
func (d *Decoder) RawChunks() []RawChunk {
	if d == nil {
		return nil
	}

	return cloneRawChunks(d.UnknownChunks)
}

// SetRawChunks replaces preserved non-core chunks with the provided set.
func (d *Decoder) SetRawChunks(chunks []RawChunk) {
	if d == nil {
		return
	}

	d.UnknownChunks = cloneRawChunks(chunks)
}

// FormatChunk returns a copy of the configured fmt chunk, if available.
func (e *Encoder) FormatChunk() *FmtChunk {
	if e == nil || e.FmtChunk == nil {
		return nil
	}

	return e.FmtChunk.Clone()
}

// RawChunks returns a copy of configured non-core chunks.
func (e *Encoder) RawChunks() []RawChunk {
	if e == nil {
		return nil
	}

	return cloneRawChunks(e.UnknownChunks)
}

// SetRawChunks replaces configured non-core chunks with the provided set.
func (e *Encoder) SetRawChunks(chunks []RawChunk) {
	if e == nil {
		return
	}

	e.UnknownChunks = cloneRawChunks(chunks)
}
