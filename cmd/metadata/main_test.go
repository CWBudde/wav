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

func TestRunNoMetadata(t *testing.T) {
	var outBuf bytes.Buffer
	err := run([]string{"../../fixtures/kick.wav"}, &outBuf)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	out := outBuf.String()
	if !strings.Contains(out, "No metadata present") {
		t.Fatalf("expected 'No metadata present' in output, got:\n%s", out)
	}
}

func TestRunInvalidPath(t *testing.T) {
	var outBuf bytes.Buffer
	err := run([]string{"/nonexistent/path.wav"}, &outBuf)
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestRunPrintsAllMetadataFields(t *testing.T) {
	var outBuf bytes.Buffer
	err := run([]string{"../../fixtures/listinfo.wav"}, &outBuf)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	out := outBuf.String()
	fields := []string{
		"Artist:",
		"Title:",
		"Comments:",
		"Copyright:",
		"CreationDate:",
		"Engineer:",
		"Technician:",
		"Genre:",
		"Keywords:",
		"Medium:",
		"Product:",
		"Subject:",
		"Software:",
		"Source:",
		"Location:",
		"TrackNbr:",
		"Sample Info:",
	}

	for _, field := range fields {
		if !strings.Contains(out, field) {
			t.Fatalf("expected output to contain %q\nfull output:\n%s", field, out)
		}
	}
}
