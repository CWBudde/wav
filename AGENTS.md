# Repository Guidelines

## Project Structure & Module Organization

- Root Go package files live at the repo top level (e.g., `wav.go`, `decoder.go`, `encoder.go`).
- Command-line tools are under `cmd/` with one folder per tool (e.g., `cmd/metadata`, `cmd/wavtoaiff`).
- Tests use standard Go `_test.go` files beside the code (e.g., `decoder_test.go`, `metadata_test.go`).
- Binary test fixtures and sample assets live in `fixtures/` (WAV/AIFF/etc.).

## Build, Test, and Development Commands

- `go test ./...` runs the full test suite across all packages.
- `go test ./... -run TestName` runs a targeted test.
- `go vet ./...` performs static analysis and common correctness checks.
- `go build ./cmd/wavtoaiff` builds a specific CLI tool (repeat for other `cmd/*`).

## Coding Style & Naming Conventions

- Use `gofmt` for formatting (tabs for indentation, standard Go layout).
- Follow Go naming: exported identifiers in `CamelCase`, unexported in `camelCase`.
- Keep file names short and descriptive; tests are named `*_test.go`.
- Prefer small, focused functions; WAV chunk logic is typically isolated per file (e.g., `cue_chunk.go`).

## Testing Guidelines

- Tests are written with the Go `testing` package.
- Name tests `TestXxx` and benchmarks `BenchmarkXxx`.
- Use fixtures from `fixtures/` for I/O-heavy tests; avoid adding large new assets unless needed.
- No explicit coverage target is enforced; prioritize meaningful casejust s and edge conditions.

## Commit & Pull Request Guidelines

- Commit messages in history are short, imperative, and topic-focused (e.g., “go fmt”, “Use io and os instead of ioutil”).
- Keep commits scoped to a single change; avoid mixing refactors with behavior changes.
- PRs should include a clear description of the change, test results, and any fixture additions.

## Agent-Specific Notes

- Prefer minimal, surgical edits; avoid reformatting unrelated files.
- Keep this repo Go-version compatible with `go 1.22` unless explicitly updated.
