# WAV Format Roadmap (PLAN)

## Goals

1. Achieve robust, standards-aware WAV parsing/writing across PCM, IEEE float, G.711, and extensible WAV variants.
2. Replace placeholder compressed-format behavior with explicit codec implementations or explicit unsupported behavior.
3. Preserve and round-trip non-audio chunk content wherever possible.
4. Expand metadata/chunk support to match real-world tooling interoperability.
5. Add compatibility-focused tests that validate round-trip chunk integrity, not only sample data.

## Current State Snapshot

- Core decode/encode support exists for PCM, IEEE float, A-law, mu-law.
- `WAVE_FORMAT_EXTENSIBLE` subtype is recognized for some decode paths.
- `LIST` parser currently handles INFO subset and skips `adtl` (`list_chunk.go`).
- Unknown chunks are drained and discarded during parse; not preserved for write.
- Encoder always writes classic 16-byte `fmt ` chunk (not extensible-aware).
- GSM/TrueSpeech/Voxware currently use deterministic fallback waveform generation (not bit-accurate codec decode).

---

## Workstreams

## 1) `fmt ` Chunk Model & Extensible Read/Write

### Why

Current handling parses only part of extensible fields and does not preserve them for write. This causes metadata/format loss and limits interoperability.

### Deliverables

1. Introduce explicit `FmtChunk` model in Go with:

- base fields: format tag, channels, sample rate, avg bytes/sec, block align, bits/sample
- extensible fields: valid bits/sample, channel mask, subformat GUID
- raw extension bytes (for unknown future-compatible payload)

2. Decoder:

- parse and store all `fmt ` fields (classic + extensible)
- set effective codec format from subformat when extensible
- preserve original raw extension bytes

3. Encoder:

- write classic `fmt ` for plain PCM/float cases only when appropriate
- write extensible `fmt ` when required (non-PCM/float subtype mapping, channel mask, valid bits)
- preserve/emit known extension values when round-tripping

### Target Files

- `decoder.go`
- `encoder.go`
- new file e.g. `fmt_chunk.go`
- tests in `decoder_test.go`, `encoder_test.go`

### Acceptance Criteria

- Extensible fixtures decode with preserved valid bits/channel mask/subformat.
- Re-encoded extensible files contain extensible `fmt ` where required.
- No regressions for classic 16-byte `fmt ` fixtures.

---

## 2) Remove Pseudo Compressed Decode; Add Correct Strategy

### Why

Current GSM/TrueSpeech/Voxware path produces pseudo-waveform output and is not true decoding. This can mislead downstream users.

### Deliverables

Choose one policy and apply consistently:

Option A (preferred): real decode

- Implement true decoders for:
  - GSM 6.10 (format 49)
  - TrueSpeech (format 34)
  - Voxware (format 6172)
- If fully correct decode for TrueSpeech/Voxware is infeasible, keep explicit unsupported for those with clear errors.

Option B: explicit unsupported (safe interim)

- Remove pseudo decode path.
- Return deterministic, explicit errors for unsupported compressed formats.
- Keep sample count/fact parsing utilities for diagnostics only.

### Target Files

- `decoder.go`
- tests: `decoder_test.go`

### Acceptance Criteria

- No pseudo decode remains.
- Each format is either bit-meaningful decoded or explicitly rejected.
- Tests assert behavior per codec.

---

## 3) Unknown Chunk Preservation & Round-Trip

### Why

Dropping unknown chunks destroys important metadata and prevents lossless-ish transcode workflows.

### Deliverables

1. Add raw chunk container model:

- chunk ID
- size
- data bytes
- original order index

2. Decoder:

- store all non-core chunks as raw if not fully parsed by typed handlers
- keep original order relative to core chunks where practical

3. Encoder:

- write preserved unknown chunks back during save
- maintain word alignment and padding correctness

### Target Files

- `decoder.go`
- `encoder.go`
- new file e.g. `chunk_model.go`

### Acceptance Criteria

- Fixture with custom chunk round-trips with chunk byte payload unchanged.
- Known chunk decode paths still function.

---

## 4) `LIST/adtl` Support (`labl`, `note`, `ltxt`)

### Why

Cue labels/notes are widely used in DAWs and currently ignored.

### Deliverables

1. Parse `LIST` subtype `adtl`.
2. Support subchunks:

- `labl`
- `note`
- `ltxt`

3. Bind entries to cue IDs in metadata model.
4. Add write support for `adtl` when metadata contains labeled cues.

### Target Files

- `list_chunk.go`
- `metadata.go`
- `encoder.go`
- tests: `metadata_test.go`, `decoder_test.go`, `encoder_test.go`

### Acceptance Criteria

- A fixture containing cue+adtl preserves labels on read.
- Re-encode keeps adtl semantics.

---

## 5) Broadcast/Cart Metadata Expansion (`bext`, `cart`)

### Why

Interoperability with broadcast and radio workflows depends on these chunks.

### Deliverables

1. Add metadata structs for:

- `bext` core fields (description, originator, refs, origination date/time, time refs, version, UMID/reserved as needed)
- `cart` practical fields used in common workflows

2. Decoder support:

- parse these chunks into metadata model

3. Encoder support:

- optional writing when metadata fields are set

### Target Files

- `metadata.go`
- `decoder.go`
- new chunk parsers e.g. `bext_chunk.go`, `cart_chunk.go`
- `encoder.go`

### Acceptance Criteria

- Fixtures with `bext`/`cart` parse into metadata.
- Write-read roundtrip preserves values.

---

## 6) Chunk Registry / Typed Chunk Architecture

### Why

Current parser logic is monolithic. A registry-based approach improves extensibility and maintenance.

### Deliverables

1. Introduce chunk handler interface:

- `CanHandle(chunkID, listType?)`
- `Decode(...)`
- `Encode(...)` (where relevant)

2. Register handlers for built-in chunks (`fmt`, `fact`, `data`, `LIST`, `cue`, `smpl`, `bext`, `cart`, etc.).
3. Unknown handler fallback to raw preservation.

### Target Files

- new files e.g. `chunk_registry.go`, `chunk_handlers.go`
- integrate in `decoder.go`, `encoder.go`

### Acceptance Criteria

- Parser behavior matches existing fixtures.
- Adding a new chunk handler requires minimal boilerplate.

---

## 7) Compatibility & Round-Trip Test Expansion

### Why

Current tests are mostly sample-centric. Need compatibility-level guarantees.

### Deliverables

1. Add chunk inventory test helper:

- list chunk IDs, sizes, order
- compare before/after re-encode

2. Golden tests:

- extensible fmt retention
- unknown chunk retention
- list/adtl retention
- bext/cart retention

3. Codec behavior tests:

- supported codecs decode/encode expectations
- unsupported codecs explicit error messages

4. Streaming parity tests:

- `PCMBuffer` vs `FullPCMBuffer` equivalence for all supported formats

### Target Files

- `decoder_test.go`
- `encoder_test.go`
- new test helpers e.g. `chunk_test_helpers.go`

### Acceptance Criteria

- `go test ./...` passes with expanded matrix.
- New tests detect format/chunk-loss regressions.

---

## 8) API Surface Improvements

### Why

As metadata/chunk coverage grows, API should expose data cleanly.

### Deliverables

1. Add optional API methods:

- `FormatChunk() *FmtChunk`
- `RawChunks() []RawChunk`
- `SetRawChunks(...)`

2. Keep backward compatibility:

- existing `Decoder`/`Encoder` fields and methods continue working
- introduce additive API only

### Acceptance Criteria

- Existing users compile unchanged.
- New APIs are documented and tested.

---

## Prioritization & Milestones

## Milestone 1 (Correctness Baseline)

1. `fmt ` model + extensible preservation/read-write.
2. Remove pseudo compressed decode OR replace with true decode for codecs actually implemented.
3. Unknown chunk retention basic support.

## Milestone 2 (Interoperability)

1. `LIST/adtl` support.
2. `bext` read/write.
3. chunk-inventory roundtrip tests.

## Milestone 3 (Advanced Metadata)

1. `cart` read/write.
2. registry refactor.
3. API polish and docs.

---

## Risks and Mitigations

1. Risk: Breaking existing simple workflows with heavy refactor.

- Mitigation: additive changes, keep legacy method behavior, strong regression tests.

2. Risk: Incorrect codec decode for legacy compressed formats.

- Mitigation: prefer explicit unsupported over fake decode; only ship real decoders with known-good vectors.

3. Risk: Chunk order/padding bugs.

- Mitigation: chunk inventory tests + byte-alignment tests.

4. Risk: Metadata field ambiguity (spec variants).

- Mitigation: preserve raw bytes where semantic parse is uncertain.

---

## Definition of Done (Overall)

1. No pseudo decode paths remain.
2. Extensible `fmt ` fields are preserved and correctly written.
3. Unknown chunks are round-tripped.
4. `adtl` and key metadata chunks (`bext`, optional `cart`) are supported.
5. Compatibility tests validate chunk retention and streaming/full decode parity.
6. `go test ./...` passes.

---

## Immediate Next Action

Start Milestone 1 with this sequence:

1. Introduce `FmtChunk` model + wire decoder parser.
2. Wire encoder `fmt ` writing logic (classic vs extensible).
3. Add extensible roundtrip tests.
4. Replace current pseudo compressed decode with explicit unsupported until real decoders land.
