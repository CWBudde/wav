package wav

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/go-audio/riff"
)

var errChunkEncodeNotSupported = errors.New("chunk encode not supported")

// ChunkHandler is a typed handler for RIFF/WAV chunks.
// Encode is optional and may return errChunkEncodeNotSupported.
type ChunkHandler interface {
	CanHandle(chunkID [4]byte, listType [4]byte) bool
	Decode(d *Decoder, ch *riff.Chunk) error
	Encode(e *Encoder) error
}

// ChunkRegistry resolves chunks to handlers.
type ChunkRegistry struct {
	handlers []ChunkHandler
}

func newDefaultChunkRegistry() *ChunkRegistry {
	return &ChunkRegistry{
		handlers: []ChunkHandler{
			&factChunkHandler{},
			&listChunkHandler{},
			&smplChunkHandler{},
			&cueChunkHandler{},
			&bextChunkHandler{},
			&cartChunkHandler{},
		},
	}
}

// Register appends a handler to the registry.
func (r *ChunkRegistry) Register(handler ChunkHandler) {
	if r == nil || handler == nil {
		return
	}

	r.handlers = append(r.handlers, handler)
}

// Decode dispatches a chunk to the first matching handler.
func (r *ChunkRegistry) Decode(dec *Decoder, chnk *riff.Chunk) (bool, error) {
	if r == nil || chnk == nil {
		return false, nil
	}

	listType, err := sniffListType(chnk)
	if err != nil {
		return false, err
	}

	for _, handler := range r.handlers {
		if handler.CanHandle(chnk.ID, listType) {
			err := handler.Decode(dec, chnk)
			if err != nil {
				return true, fmt.Errorf("chunk handler decode failed: %w", err)
			}

			return true, nil
		}
	}

	return false, nil
}

func sniffListType(chnk *riff.Chunk) ([4]byte, error) {
	var listType [4]byte

	if chnk == nil || chnk.ID != CIDList || chnk.Size < 4 {
		return listType, nil
	}

	var head [4]byte

	n, err := io.ReadFull(chnk.R, head[:])
	if err != nil {
		return listType, fmt.Errorf("failed to read LIST type: %w", err)
	}

	copy(listType[:], head[:])

	remaining := io.LimitReader(chnk.R, int64(chnk.Size-n))
	chnk.R = io.MultiReader(bytes.NewReader(head[:]), remaining)

	return listType, nil
}

type factChunkHandler struct{}

func (h *factChunkHandler) CanHandle(chunkID [4]byte, _ [4]byte) bool {
	return chunkID == CIDFact
}

func (h *factChunkHandler) Decode(dec *Decoder, chunk *riff.Chunk) error {
	if dec == nil || chunk == nil {
		return nil
	}

	var sampleCount uint32

	err := chunk.ReadLE(&sampleCount)
	if err == nil {
		dec.CompressedSamples = sampleCount
	}

	chunk.Drain()

	return nil
}

func (h *factChunkHandler) Encode(_ *Encoder) error {
	return errChunkEncodeNotSupported
}

type listChunkHandler struct{}

func (h *listChunkHandler) CanHandle(chunkID [4]byte, _ [4]byte) bool {
	return chunkID == CIDList
}

func (h *listChunkHandler) Decode(d *Decoder, ch *riff.Chunk) error {
	return DecodeListChunk(d, ch)
}

func (h *listChunkHandler) Encode(_ *Encoder) error {
	return errChunkEncodeNotSupported
}

type smplChunkHandler struct{}

func (h *smplChunkHandler) CanHandle(chunkID [4]byte, _ [4]byte) bool {
	return chunkID == CIDSmpl
}

func (h *smplChunkHandler) Decode(d *Decoder, ch *riff.Chunk) error {
	return DecodeSamplerChunk(d, ch)
}

func (h *smplChunkHandler) Encode(_ *Encoder) error {
	return errChunkEncodeNotSupported
}

type cueChunkHandler struct{}

func (h *cueChunkHandler) CanHandle(chunkID [4]byte, _ [4]byte) bool {
	return chunkID == CIDCue
}

func (h *cueChunkHandler) Decode(d *Decoder, ch *riff.Chunk) error {
	return DecodeCueChunk(d, ch)
}

func (h *cueChunkHandler) Encode(_ *Encoder) error {
	return errChunkEncodeNotSupported
}

type bextChunkHandler struct{}

func (h *bextChunkHandler) CanHandle(chunkID [4]byte, _ [4]byte) bool {
	return chunkID == CIDBext
}

func (h *bextChunkHandler) Decode(d *Decoder, ch *riff.Chunk) error {
	return DecodeBroadcastChunk(d, ch)
}

func (h *bextChunkHandler) Encode(e *Encoder) error {
	if e == nil || e.Metadata == nil || e.Metadata.BroadcastExtension == nil {
		return nil
	}

	return e.writeRawChunk(RawChunk{ID: CIDBext, Data: encodeBroadcastChunk(e.Metadata.BroadcastExtension)})
}

type cartChunkHandler struct{}

func (h *cartChunkHandler) CanHandle(chunkID [4]byte, _ [4]byte) bool {
	return chunkID == CIDCart
}

func (h *cartChunkHandler) Decode(d *Decoder, ch *riff.Chunk) error {
	return DecodeCartChunk(d, ch)
}

func (h *cartChunkHandler) Encode(e *Encoder) error {
	if e == nil || e.Metadata == nil || e.Metadata.Cart == nil {
		return nil
	}

	return e.writeRawChunk(RawChunk{ID: CIDCart, Data: encodeCartChunk(e.Metadata.Cart)})
}
