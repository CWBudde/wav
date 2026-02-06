package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cwbudde/wav"
)

func TestTagFileWritesMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	inPath := filepath.Join(tmpDir, "sample_title.wav")

	data, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "kick.wav"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	if err := os.WriteFile(inPath, data, 0o644); err != nil {
		t.Fatalf("write temp input: %v", err)
	}

	*flagArtist = "Test Artist"
	*flagTitleRegexp = "^sample_(.*)$"
	*flagTitle = ""
	*flagComments = "Comment"
	*flagCopyright = "Copyright"
	*flagGenre = "Genre"
	defer func() {
		*flagArtist = ""
		*flagTitleRegexp = ""
		*flagTitle = ""
		*flagComments = ""
		*flagCopyright = ""
		*flagGenre = ""
	}()

	if err := tagFile(inPath); err != nil {
		t.Fatalf("tagFile returned error: %v", err)
	}

	outPath := filepath.Join(tmpDir, "wavtagger", "sample_title.wav")
	outFile, err := os.Open(outPath)
	if err != nil {
		t.Fatalf("open tagged file: %v", err)
	}
	defer outFile.Close()

	dec := wav.NewDecoder(outFile)
	dec.ReadMetadata()
	if err := dec.Err(); err != nil {
		t.Fatalf("decoder error: %v", err)
	}

	if dec.Metadata == nil {
		t.Fatalf("expected metadata to be present")
	}

	if dec.Metadata.Artist != "Test Artist" {
		t.Fatalf("artist=%q, want %q", dec.Metadata.Artist, "Test Artist")
	}

	if dec.Metadata.Title != "title" {
		t.Fatalf("title=%q, want %q", dec.Metadata.Title, "title")
	}

	if dec.Metadata.Comments != "Comment" {
		t.Fatalf("comments=%q, want %q", dec.Metadata.Comments, "Comment")
	}

	if dec.Metadata.Copyright != "Copyright" {
		t.Fatalf("copyright=%q, want %q", dec.Metadata.Copyright, "Copyright")
	}

	if dec.Metadata.Genre != "Genre" {
		t.Fatalf("genre=%q, want %q", dec.Metadata.Genre, "Genre")
	}
}

func TestTagFileMissingInput(t *testing.T) {
	err := tagFile(filepath.Join(t.TempDir(), "missing.wav"))
	if err == nil {
		t.Fatalf("expected an error for missing input file")
	}
}
