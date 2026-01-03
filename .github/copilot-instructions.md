# WAV Codec Library - AI Coding Instructions

## Project Overview

This is a Go library for encoding and decoding WAV audio files, part of the `github.com/go-audio` ecosystem. It handles PCM audio data, metadata (LIST/INFO chunks), sampler information (smpl chunks), and cue points.

## Architecture

### Core Components

- **Decoder** ([decoder.go](decoder.go)) - Reads WAV files using the RIFF parser from `github.com/go-audio/riff`
- **Encoder** ([encoder.go](encoder.go)) - Writes WAV files with PCM data and metadata
- **Metadata** ([metadata.go](metadata.go)) - Handles LIST/INFO chunks and sampler-specific data
- **Chunk Decoders** - Specialized parsers for smpl ([smpl_chunk.go](smpl_chunk.go)), LIST ([list_chunk.go](list_chunk.go)), and cue ([cue_chunk.go](cue_chunk.go)) chunks

### External Dependencies

- `github.com/go-audio/riff` - RIFF container parsing (WAV files use RIFF format)
- `github.com/go-audio/audio` - Audio buffer abstraction (`audio.IntBuffer` for normalized PCM data)
- `github.com/go-audio/aiff` - Only used in cmd/wavtoaiff converter tool

## Critical Patterns

### Decoder Workflow

1. Create decoder with `NewDecoder(io.ReadSeeker)` - requires seekable reader
2. Call `ReadInfo()` to parse format chunk (sample rate, bit depth, channels)
3. Call `FwdToPCM()` to advance to PCM data chunk
4. Read audio using `PCMBuffer(*audio.IntBuffer)` (efficient) or `FullPCMBuffer()` (loads all into memory)
5. Use `Rewind()` for re-reading the same file - creates new parser and resets state

### Encoder Workflow

1. Create with `NewEncoder(io.WriteSeeker, sampleRate, bitDepth, numChans, audioFormat)`
2. Set `encoder.Metadata` if needed (must be before writing frames)
3. Write audio data via `Write(*audio.IntBuffer)` or `WriteFrame(interface{})`
4. Call `Close()` to finalize - updates chunk sizes in header by seeking back

### Binary Encoding Convention

- **Little-endian** for all WAV data fields (via `binary.LittleEndian`)
- **Big-endian** only for RIFF chunk IDs (4-byte markers like 'RIFF', 'fmt ', 'data')
- Use `encoder.AddLE()` and `encoder.AddBE()` helpers for consistency

### Bit Depth Handling

Supported: 8, 16, 24, 32-bit PCM. See [encoder.go](encoder.go#L70-L93) `addBuffer()` for per-bit-depth encoding:

- 8-bit: unsigned (`uint8`)
- 16-bit: signed (`int16`)
- 24-bit: special 3-byte encoding via `audio.Int32toInt24LEBytes()`
- 32-bit: signed (`int32`)

### Metadata Structure

Metadata is optional and stored in separate chunks:

- **LIST/INFO chunks** - Text fields like Artist, Title, Comments (see [list_chunk.go](list_chunk.go) markers: `IART`, `INAM`, etc.)
- **smpl chunk** - Sampler info: MIDI notes, loops, SMPTE offsets (see [smpl_chunk.go](smpl_chunk.go))
- **cue chunk** - Cue points for markers/regions (see [cue_chunk.go](cue_chunk.go))

Decode chunks via `DecodeListChunk()`, `DecodeSamplerChunk()`, `DecodeCueChunk()` during `ReadInfo()`.

## Testing Conventions

### Test Files

All fixtures in `fixtures/` directory: `kick.wav`, `bass.wav`, `dirty-kick-24b441k.wav`, `bwf.wav`, etc.
Invalid files for negative testing: `sample.avi`, `bloop.aif`

### Test Patterns

- Use table-driven tests (see [decoder_test.go](decoder_test.go#L64-L72))
- Test with actual fixture files, not mocked data
- Verify round-trip encoding/decoding produces identical data
- Test `Rewind()` by comparing buffers before/after (see [decoder_test.go](decoder_test.go#L30-L56))

### Running Tests

```bash
go test                    # Run all tests
go test -v                 # Verbose output
go test -run TestName      # Specific test
```

## Common Pitfalls

1. **Forgot to call `encoder.Close()`** - WAV headers contain data sizes that must be written after encoding completes
2. **Using `FullPCMBuffer()` for large files** - Loads entire audio into memory; use `PCMBuffer()` for streaming
3. **Not checking `decoder.Err()`** - Errors during parsing are cached; always check after operations
4. **Mixing endianness** - WAV spec requires little-endian for data, big-endian for chunk IDs
5. **Assuming 16-bit samples** - Always check `decoder.BitDepth` before processing

## Command-line Tools

Examples in `cmd/`:

- `gen-sine` - Generate sine wave WAV files (demonstrates encoder usage)
- `metadata` - Read/write metadata chunks
- `wavtagger` - Tag WAV files with metadata
- `wavtoaiff` - Convert WAV to AIFF format

Run with: `go run cmd/gen-sine/main.go -output test.wav -frequency 440 -lenght 5`

## Code Style

- Use `fmt.Errorf` with `%w` for error wrapping
- Nil checks before dereferencing in all public methods
- Export types/functions start with uppercase; internal with lowercase
- Validate inputs early (e.g., nil buffer checks in `encoder.addBuffer()`)
