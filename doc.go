// Package wav provides WAV encoding and decoding utilities for Go.
//
// The package supports PCM integer (8/16/24/32-bit), IEEE float
// (32/64-bit), A-law, mu-law, and GSM 6.10 decode paths. It also parses and
// encodes common WAV metadata chunks, including LIST/INFO, cue/smpl, bext,
// and cart.
//
// For chunk-preserving round-trip workflows, Decoder and Encoder expose
// additive APIs:
//
//   - FormatChunk() *FmtChunk
//   - RawChunks() []RawChunk
//   - SetRawChunks([]RawChunk)
//
// Existing Decoder/Encoder fields and methods remain supported for backward
// compatibility.
package wav
