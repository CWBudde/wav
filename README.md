# wav

[![Go Reference](https://pkg.go.dev/badge/github.com/cwbudde/wav.svg)](https://pkg.go.dev/github.com/cwbudde/wav)
[![Go Report Card](https://goreportcard.com/badge/github.com/cwbudde/wav)](https://goreportcard.com/report/github.com/cwbudde/wav)

A Go package for encoding and decoding [WAV](https://en.wikipedia.org/wiki/WAV) audio files. Supports PCM integer (8/16/24/32-bit) and IEEE float (32/64-bit) formats, metadata (LIST/INFO chunks), sampler info (smpl chunks), and cue points.

> Note: This repository is a fork from [go-audio/wav](https://github.com/go-audio/wav)

## Installation

```sh
go get github.com/cwbudde/wav
```

## Features

- Decode WAV files into `audio.Float32Buffer` (via [go-audio/audio](https://github.com/go-audio/audio))
- Encode `audio.Float32Buffer` data into valid WAV files
- Read and write LIST/INFO metadata (artist, title, genre, comments, etc.)
- Read sampler information (loops, MIDI note, SMPTE offset)
- Read cue points
- Streaming decoding with `PCMBuffer` for memory-efficient processing
- Rewind support for looped playback

## Usage

### Decoding a WAV file

```go
file, err := os.Open("input.wav")
if err != nil {
    log.Fatal(err)
}
defer file.Close()

decoder := wav.NewDecoder(file)
buf, err := decoder.FullPCMBuffer()
if err != nil {
    log.Fatal(err)
}

fmt.Printf("%d channels, %d Hz, %d-bit\n",
    decoder.NumChans, decoder.SampleRate, decoder.BitDepth)
fmt.Printf("%d samples decoded\n", len(buf.Data))
```

### Streaming decode (memory-efficient)

```go
decoder := wav.NewDecoder(file)

buf := &audio.Float32Buffer{Data: make([]float32, 4096)}
for {
    n, err := decoder.PCMBuffer(buf)
    if n == 0 {
        break
    }
    if err != nil {
        log.Fatal(err)
    }
    // process buf.Data[:n]
}
```

### Encoding a WAV file

```go
out, err := os.Create("output.wav")
if err != nil {
    log.Fatal(err)
}
defer out.Close()

encoder := wav.NewEncoder(out, 44100, 16, 2, 1) // sampleRate, bitDepth, channels, PCM format
if err := encoder.Write(buf); err != nil {
    log.Fatal(err)
}
if err := encoder.Close(); err != nil {
    log.Fatal(err)
}
```

### Reading metadata

```go
decoder := wav.NewDecoder(file)
decoder.ReadMetadata()

if decoder.Metadata != nil {
    fmt.Println("Artist:", decoder.Metadata.Artist)
    fmt.Println("Title:", decoder.Metadata.Title)
    fmt.Println("Genre:", decoder.Metadata.Genre)
}
```

### Writing metadata

```go
encoder := wav.NewEncoder(out, 44100, 16, 2, 1)
encoder.Metadata = &wav.Metadata{
    Artist: "Artist Name",
    Title:  "Track Title",
    Genre:  "Electronic",
}
encoder.Write(buf)
encoder.Close()
```

## CLI Tools

The `cmd/` directory contains several command-line utilities:

| Tool            | Description                                        |
| --------------- | -------------------------------------------------- |
| `cmd/metadata`  | Read and display metadata from a WAV file          |
| `cmd/wavtoaiff` | Convert a WAV file to AIFF format                  |
| `cmd/wavtagger` | Tag WAV files with metadata (single file or batch) |
| `cmd/gen-sine`  | Generate a sine wave WAV file at a given frequency |

### Examples

```sh
# Read metadata
go run ./cmd/metadata fixtures/listinfo.wav

# Convert WAV to AIFF
go run ./cmd/wavtoaiff -path input.wav

# Tag a WAV file
go run ./cmd/wavtagger -file input.wav -artist "Name" -title "Song"

# Tag all WAV files in a directory
go run ./cmd/wavtagger -dir ./samples -genre "Drums" -regexp 'kit_\d+_(.*)'

# Generate a 440 Hz sine wave (5 seconds)
go run ./cmd/gen-sine -output sine.wav -frequency 440 -length 5
```

## Supported Metadata Fields

The `Metadata` struct supports standard LIST/INFO chunk fields:

Artist, Title, Comments, Copyright, CreationDate, Engineer, Technician, Genre, Keywords, Medium, Product (album), Subject, Software, Source, Location, TrackNbr

Additionally, `SamplerInfo` provides MIDI and loop metadata from the smpl chunk.

## API Documentation

See the full API reference on [pkg.go.dev](https://pkg.go.dev/github.com/cwbudde/wav).

## Round-trip APIs

For metadata/chunk-preserving workflows, the decoder and encoder now expose:

- `FormatChunk() *FmtChunk`
- `RawChunks() []RawChunk`
- `SetRawChunks([]RawChunk)`

These methods are additive and coexist with existing fields/methods for backward compatibility.
