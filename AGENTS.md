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

## Releasing, and Not Drifting

This module is part of the `github.com/cwbudde/algo-*` family, which is co-developed
across separate repositories. That arrangement failed once already, and the rules below
exist to stop it failing the same way twice.

**What went wrong (August 2026).** The family had drifted onto three different `algo-fft`
versions simultaneously — `algo-pde` on v0.6.15, `algo-dsp` on v0.7.3, `algo-acoustics` on
v0.6.11 — while `algo-fft`'s own `main` sat 97 commits past its latest tag and its
CHANGELOG documented a `v0.7.5` that had never been tagged. Because `algo-fft`'s generic
`PlanReal2D`/`PlanReal3D` had changed signature between the v0.6 and v0.7 lines, _no single
upgrade anywhere would compile_. Untangling it took a day and four coordinated releases.

Three separate mistakes combined to produce that. Each now has a check.

### 1. Do not let work pile up untagged

Work that only exists on `main` cannot be consumed. If you finish something a sibling repo
needs, tag it — do not wait for a milestone.

```bash
just check-unreleased     # how much is sitting past the latest tag?
```

A scheduled CI job (`.github/workflows/dep-drift.yml`) reports this weekly.

### 2. Do not sit on stale siblings

```bash
just check-deps           # are all github.com/cwbudde/* deps at their latest tags?
```

This is wired into the repo's aggregate check recipe, and the same scheduled job files a
GitHub issue when it starts failing. If a bump is _deliberately_ deferred, write down why in
`PLAN.md` — an undocumented old pin is indistinguishable from a forgotten one.

Renovate (`.github/renovate.json`) opens the bump PRs automatically and groups the whole
`cwbudde` family into a single PR on purpose: an incompatible `algo-fft` can reach a
consumer through two different dependency paths at once, so bumping them one PR at a time
produces intermediate combinations that never build.

### 3. Never remove or change exported API without the version saying so

Always release through the guard rather than by hand:

```bash
just tag-release v0.8.0       # runs every precondition, then tags and pushes
```

It refuses to tag when the tree is dirty, when `HEAD` is not a pushed default branch, when the tag
already exists or does not sort after the current one, when siblings are stale, when
`CHANGELOG.md` has no section for the version, or when the exported API changed
incompatibly without the version reflecting it.

**That last rule is stricter than semver, deliberately.** Semver exempts `v0.x` — "anything
MAY change at any time" — so `gorelease` will happily approve a _patch_ bump across a
removed symbol. Every module in this family is `v0.x`, so that exemption is exactly the hole
we fell through: `KernelEightStep` was removed and `PlanReal2D` became generic, and nothing
in the version numbers said so. The guard therefore requires a **minor** bump for any
incompatible change while on `v0.x`.

When you do break API, say so in the CHANGELOG in the form a consumer needs: the old
signature, the new signature, and the call-site rewrite. "Refactored plans" does not help
anyone; `NewPlanReal2D(rows, cols)` → `NewPlanReal2D64(rows, cols)` does.

### Order of operations for a cross-repo change

Releases must flow up the dependency graph, never sideways:

```
algo-vecmath ─┐
algo-approx  ─┼─→ algo-dsp ─┐
algo-fft ─────┴─→ algo-pde ─┴─→ algo-acoustics
```

Tag the dependency first, then bump and tag its consumers, then the consumers' consumers.
Bumping a consumer before its dependency is tagged is what forces pseudo-versions into
`go.mod`, and those are how a repo quietly ends up pinned to a commit nobody can find later.
