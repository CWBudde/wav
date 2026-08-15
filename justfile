# Build the library
build:
    go build -v ./...

# Run all tests
test:
    go test -v -race -count=1 ./...

# Run benchmarks
bench:
    go test -bench=. -benchmem -run=^$ ./...

# Run linters
lint:
    GOFLAGS="-buildvcs=false" golangci-lint run

# Run linters and fix issues
lint-fix:
    GOFLAGS="-buildvcs=false" golangci-lint run --fix

# Format code using treefmt
fmt:
    treefmt . --allow-missing-formatter

# Check if code is formatted
fmt-check:
    treefmt --allow-missing-formatter --fail-on-change

# Generate coverage report
cover:
    go test -coverprofile=coverage.txt -covermode=atomic ./...
    go tool cover -html=coverage.txt -o coverage.html

# Clean build artifacts
clean:
    rm -f coverage.txt coverage.html

# Run all checks (test, lint, coverage)
check: test lint cover check-deps

# Default target
default: build

fix:
    just lint-fix
    just fmt

# Are all github.com/cwbudde/* dependencies at their latest tags?
check-deps:
    ./scripts/release-guard.sh deps

# How much work is sitting on main past the latest tag?
check-unreleased:
    ./scripts/release-guard.sh unreleased

# Check every release precondition for VERSION without tagging anything.
release-check VERSION:
    ./scripts/release-guard.sh gate {{VERSION}}

# Tag VERSION: run the full gate, then create and push the annotated tag.
# Refuses on a dirty tree, stale siblings, a missing CHANGELOG section, or an
# incompatible API change the version does not signal. See AGENTS.md.
tag-release VERSION:
    ./scripts/release-guard.sh tag {{VERSION}}
