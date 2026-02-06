package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestRunRequiresPath(t *testing.T) {
	var out bytes.Buffer
	err := run(nil, &out)
	if err == nil {
		t.Fatalf("expected error without input path")
	}

	if !errors.Is(err, errMissingPath) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPrintsMetadataWithoutPanic(t *testing.T) {
	var outBuf bytes.Buffer
	err := run([]string{"../../fixtures/listinfo.wav"}, &outBuf)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	out := outBuf.String()
	checks := []string{
		"Artist: artist",
		"Title: track title",
		"Comments: my comment",
		"TrackNbr: 42",
		"Sample Info:",
	}

	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Fatalf("expected output to contain %q\nfull output:\n%s", c, out)
		}
	}
}
