// This command line tool helps the user tag wav files by injecting metadata in
// the file in a safe way.
// All files are copied and stored in the wavtagger folder by the original files.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cwbudde/wav"
)

var (
	flagFileToTag   = flag.String("file", "", "Path to the wave file to tag")
	flagDirToTag    = flag.String("dir", "", "Directory containing all the wav files to tag")
	flagTitleRegexp = flag.String("regexp", "", `submatch regexp to use to set the title dynamically by extracting it from the filename (ignoring the extension), example: 'my_files_\d\d_(.*)'`)
	//
	flagTitle     = flag.String("title", "", "File's title")
	flagArtist    = flag.String("artist", "", "File's artist")
	flagComments  = flag.String("comments", "", "File's comments")
	flagCopyright = flag.String("copyright", "", "File's copyright")
	flagGenre     = flag.String("genre", "", "File's genre")
	// TODO: add other supported metadata types.
)

func main() {
	flag.Parse()

	if *flagFileToTag == "" && *flagDirToTag == "" {
		fmt.Println("You need to pass -file or -dir to indicate what file or folder content to tag.")
		os.Exit(1)
	}

	if *flagFileToTag != "" {
		err := tagFile(*flagFileToTag)
		if err != nil {
			fmt.Printf("Something went wrong when tagging %s - error: %v\n", *flagFileToTag, err)
			os.Exit(1)
		}
	}

	if *flagDirToTag != "" {
		var filePath string

		fileInfos, _ := os.ReadDir(*flagDirToTag)
		for _, fi := range fileInfos {
			if strings.HasPrefix(
				strings.ToLower(filepath.Ext(fi.Name())),
				".wav") {
				filePath = filepath.Join(*flagDirToTag, fi.Name())

				err := tagFile(filePath)
				if err != nil {
					fmt.Printf("Something went wrong tagging %s - %v\n", filePath, err)
				}
			}
		}
	}
}

func tagFile(path string) error {
	in, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open %s - %w", path, err)
	}

	decoder := wav.NewDecoder(in)

	buf, err := decoder.FullPCMBuffer()
	if err != nil {
		return fmt.Errorf("couldn't read buffer %s %w", path, err)
	}

	if err := in.Close(); err != nil {
		return fmt.Errorf("failed to close input file %s: %w", path, err)
	}

	outputDir := filepath.Join(filepath.Dir(path), "wavtagger")

	outPath := filepath.Join(outputDir, filepath.Base(path))
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}

	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("couldn't create %s %w", outPath, err)
	}

	defer func() {
		cerr := out.Close()
		if cerr != nil && err == nil {
			err = fmt.Errorf("failed to close output file: %w", cerr)
		}
	}()

	encoder := wav.NewEncoder(out,
		buf.Format.SampleRate,
		int(decoder.BitDepth),
		buf.Format.NumChannels,
		int(decoder.WavAudioFormat))

	err = encoder.Write(buf)
	if err != nil {
		return fmt.Errorf("failed to write audio buffer - %w", err)
	}

	encoder.Metadata = &wav.Metadata{}
	if *flagArtist != "" {
		encoder.Metadata.Artist = *flagArtist
	}

	if *flagTitleRegexp != "" {
		filename := filepath.Base(path)
		filename = filename[:len(filename)-len(filepath.Ext(path))]
		re := regexp.MustCompile(*flagTitleRegexp)

		matches := re.FindStringSubmatch(filename)
		if len(matches) > 0 {
			encoder.Metadata.Title = matches[1]
		} else {
			fmt.Printf("No matches for title regexp %s in %s\n", *flagTitleRegexp, filename)
		}
	}

	if *flagTitle != "" {
		encoder.Metadata.Title = *flagTitle
	}

	if *flagComments != "" {
		encoder.Metadata.Comments = *flagComments
	}

	if *flagCopyright != "" {
		encoder.Metadata.Copyright = *flagCopyright
	}

	if *flagGenre != "" {
		encoder.Metadata.Genre = *flagGenre
	}

	if err := encoder.Close(); err != nil {
		return fmt.Errorf("failed to close %s - %w", outPath, err)
	}

	fmt.Println("Tagged file available at", outPath)

	return nil
}
