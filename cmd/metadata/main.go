// This tool reads metadata from the passed wav file if available.
package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/cwbudde/wav"
)

const missingPathMessage = "You must pass the pass the path of the file to decode"

func main() {
	err := run(os.Args[1:], os.Stdout)
	if err == nil {
		return
	}

	if errors.Is(err, errMissingPath) {
		fmt.Println(missingPathMessage)
		os.Exit(1)
	}

	log.Fatal(err)
}

var errMissingPath = errors.New("missing path argument")

func run(args []string, out io.Writer) (err error) {
	if len(args) < 1 {
		return errMissingPath
	}

	file, err := os.Open(args[0])
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}

	defer func() {
		cerr := file.Close()
		if cerr != nil && err == nil {
			err = cerr
		}
	}()

	dec := wav.NewDecoder(file)
	dec.ReadMetadata()

	if err := dec.Err(); err != nil {
		return fmt.Errorf("failed to read metadata: %w", err)
	}

	if dec.Metadata == nil {
		_, _ = fmt.Fprintln(out, "No metadata present")
		return nil
	}

	_, _ = fmt.Fprintf(out, "Artist: %s\n", dec.Metadata.Artist)
	_, _ = fmt.Fprintf(out, "Title: %s\n", dec.Metadata.Title)
	_, _ = fmt.Fprintf(out, "Comments: %s\n", dec.Metadata.Comments)
	_, _ = fmt.Fprintf(out, "Copyright: %s\n", dec.Metadata.Copyright)
	_, _ = fmt.Fprintf(out, "CreationDate: %s\n", dec.Metadata.CreationDate)
	_, _ = fmt.Fprintf(out, "Engineer: %s\n", dec.Metadata.Engineer)
	_, _ = fmt.Fprintf(out, "Technician: %s\n", dec.Metadata.Technician)
	_, _ = fmt.Fprintf(out, "Genre: %s\n", dec.Metadata.Genre)
	_, _ = fmt.Fprintf(out, "Keywords: %s\n", dec.Metadata.Keywords)
	_, _ = fmt.Fprintf(out, "Medium: %s\n", dec.Metadata.Medium)
	_, _ = fmt.Fprintf(out, "Product: %s\n", dec.Metadata.Product)
	_, _ = fmt.Fprintf(out, "Subject: %s\n", dec.Metadata.Subject)
	_, _ = fmt.Fprintf(out, "Software: %s\n", dec.Metadata.Software)
	_, _ = fmt.Fprintf(out, "Source: %s\n", dec.Metadata.Source)
	_, _ = fmt.Fprintf(out, "Location: %s\n", dec.Metadata.Location)
	_, _ = fmt.Fprintf(out, "TrackNbr: %s\n", dec.Metadata.TrackNbr)

	_, _ = fmt.Fprintln(out, "Sample Info:")
	_, _ = fmt.Fprintf(out, "%+v\n", dec.Metadata.SamplerInfo)

	if dec.Metadata.SamplerInfo != nil {
		for i, l := range dec.Metadata.SamplerInfo.Loops {
			_, _ = fmt.Fprintf(out, "\tloop [%d]:\t%+v\n", i, l)
		}
	}

	for i, c := range dec.Metadata.CuePoints {
		_, _ = fmt.Fprintf(out, "\tcue point [%d]:\t%+v\n", i, c)
	}

	return nil
}
