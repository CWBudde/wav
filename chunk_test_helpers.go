package wav

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
)

type testChunk struct {
	id   string
	size uint32
	data []byte
}

type chunkInventoryEntry struct {
	id   string
	size uint32
}

var (
	errFileTooSmall         = errors.New("file too small")
	errInvalidRiffWaveHdr   = errors.New("invalid riff/wave header")
	errChunkExceedsFileSize = errors.New("chunk exceeds file size")
)

func parseWavChunks(data []byte) ([]testChunk, error) {
	if len(data) < 12 {
		return nil, errFileTooSmall
	}

	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, errInvalidRiffWaveHdr
	}

	chunks := make([]testChunk, 0)

	offset := 12
	for offset+8 <= len(data) {
		id := string(data[offset : offset+4])
		size := binary.LittleEndian.Uint32(data[offset+4 : offset+8])
		offset += 8

		end := offset + int(size)
		if end > len(data) {
			return nil, fmt.Errorf("%w: %q", errChunkExceedsFileSize, id)
		}

		payload := append([]byte(nil), data[offset:end]...)
		chunks = append(chunks, testChunk{id: id, size: size, data: payload})

		offset = end
		if size%2 == 1 {
			offset++
		}
	}

	return chunks, nil
}

func parseWavChunksFromFile(path string) ([]testChunk, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return parseWavChunks(data)
}

func findChunk(chunks []testChunk, id string) (*testChunk, int) {
	for i := range chunks {
		if chunks[i].id == id {
			return &chunks[i], i
		}
	}

	return nil, -1
}

func buildChunkInventory(chunks []testChunk) []chunkInventoryEntry {
	out := make([]chunkInventoryEntry, 0, len(chunks))
	for _, ch := range chunks {
		out = append(out, chunkInventoryEntry{id: ch.id, size: ch.size})
	}

	return out
}
