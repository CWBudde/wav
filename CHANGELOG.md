# Changelog

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
- Validate raw float payload bits, PCM8 clipping and odd-chunk boundaries,
  post-data metadata, and lazy streaming buffer retention in regression tests.
