# Changelog

## [v0.1.4] - 2026-10-04

- Correct unsigned 8-bit PCM float conversion to `(byte - 128) / 128` and
  matching centered quantization. Byte 128 is exact silence; positive full scale
  is 127/128, matching signed PCM conventions. Previously the 127.5 center
  introduced DC offset. Exhaustive tests cover all 256 byte round trips.
