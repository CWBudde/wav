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

func run(args []string, out io.Writer) error {
	if len(args) < 1 {
		return errMissingPath
	}

	file, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer file.Close()

	dec := wav.NewDecoder(file)
	dec.ReadMetadata()

	if err := dec.Err(); err != nil {
		return err
	}

	if dec.Metadata == nil {
		fmt.Fprintln(out, "No metadata present")
		return nil
	}

	fmt.Fprintf(out, "Artist: %s\n", dec.Metadata.Artist)
	fmt.Fprintf(out, "Title: %s\n", dec.Metadata.Title)
	fmt.Fprintf(out, "Comments: %s\n", dec.Metadata.Comments)
	fmt.Fprintf(out, "Copyright: %s\n", dec.Metadata.Copyright)
	fmt.Fprintf(out, "CreationDate: %s\n", dec.Metadata.CreationDate)
	fmt.Fprintf(out, "Engineer: %s\n", dec.Metadata.Engineer)
	fmt.Fprintf(out, "Technician: %s\n", dec.Metadata.Technician)
	fmt.Fprintf(out, "Genre: %s\n", dec.Metadata.Genre)
	fmt.Fprintf(out, "Keywords: %s\n", dec.Metadata.Keywords)
	fmt.Fprintf(out, "Medium: %s\n", dec.Metadata.Medium)
	fmt.Fprintf(out, "Product: %s\n", dec.Metadata.Product)
	fmt.Fprintf(out, "Subject: %s\n", dec.Metadata.Subject)
	fmt.Fprintf(out, "Software: %s\n", dec.Metadata.Software)
	fmt.Fprintf(out, "Source: %s\n", dec.Metadata.Source)
	fmt.Fprintf(out, "Location: %s\n", dec.Metadata.Location)
	fmt.Fprintf(out, "TrackNbr: %s\n", dec.Metadata.TrackNbr)

	fmt.Fprintln(out, "Sample Info:")
	fmt.Fprintf(out, "%+v\n", dec.Metadata.SamplerInfo)

	if dec.Metadata.SamplerInfo != nil {
		for i, l := range dec.Metadata.SamplerInfo.Loops {
			fmt.Fprintf(out, "\tloop [%d]:\t%+v\n", i, l)
		}
	}

	for i, c := range dec.Metadata.CuePoints {
		fmt.Fprintf(out, "\tcue point [%d]:\t%+v\n", i, c)
	}

	return nil
}
