# Changelog

## v0.1.2

- Add bounded cue encoding and atomic count-validated decoding. Cue positions
  in ordinary PCM/IEEE WAV are documented as sample-frame offsets.
- Add `Metadata.AssociatedData`, `AssociatedData`, `CueLabel`, and `CueRegion`
  for LIST/adtl `labl`, `note`, and `ltxt` metadata. Decode labels independently
  of cue order; multiple associated-data lists append valid records without
  publishing partially parsed lists. Preserve unknown adtl subchunk payloads.
- Export `DecodeAssociatedDataChunk`, `EncodeCueChunk`, and
  `EncodeAssociatedDataChunk` for selective bounded decoding and exact chunk
  sizing. `Encoder.Metadata` writes cue and associated-data chunks directly.
- Bound metadata chunk allocations to 16 MiB and whole-file metadata scans to
  64 MiB. Reject malformed cue counts, nested LIST lengths, truncated records,
  missing alignment bytes, nil cue rows, and embedded NUL encoding text.
- Parse and emit word alignment per INFO/adtl subchunk. Preserve unknown LIST
  types and odd raw payloads without including RIFF padding in payload sizes.
- Scan all metadata even when earlier header metadata was already decoded;
  propagate malformed metadata errors rather than overwriting them at EOF.
- Regression coverage includes non-ASCII/odd labels, zero/large cue IDs,
  region locale fields, pre-cue/multiple adtl lists, malformed/oversized data,
  encoder integration, raw-chunk exact round-trip, and FL Studio fixture data.

## v0.1.1

- Preserve IEEE float32 and float64 sample bits when encoding, including values
  outside [-1, 1], infinities, NaN payloads, and signed zero. Decode IEEE floats
  without clipping; float32 samples retain their bits. Conversion between
  float widths follows Go's numeric conversion; integer PCM still clips.
- Add RIFF alignment padding after odd-length audio payloads, before post-data
  chunks and metadata. Data chunk sizes count only serialized sample bytes;
  RIFF sizes include alignment padding. Odd-length LIST metadata also receives
  its required alignment byte.
- Keep data-chunk padding out of decoded PCM8 and G.711 audio samples while
  advancing over it before subsequent chunks.
- Allocate the encoder's temporary buffer lazily instead of reserving one minute
  of audio at construction. Each `Write` continues to flush its samples.
- Decode streaming PCM, IEEE float, and G.711 samples directly from reusable
  byte blocks. Eliminate per-sample readers and repeated byte-buffer allocations;
  short reads are assembled into complete samples before conversion.
- Validate raw float payload bits, PCM8 clipping and odd-chunk boundaries,
  post-data metadata, and lazy streaming buffer retention in regression tests.
