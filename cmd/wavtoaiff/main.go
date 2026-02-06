// This tool converts an aiff file into an identical wav file and stores
// it in the same folder as the source.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/cwbudde/wav"
	"github.com/go-audio/aiff"
	"github.com/go-audio/audio"
)

const missingPathMessage = "You must set the -path flag"

func main() {
	err := run(os.Args[1:], user.Current, os.Stdout)
	if err == nil {
		return
	}

	if errors.Is(err, errMissingPath) {
		fmt.Println(missingPathMessage)
		os.Exit(1)
	}

	if errors.Is(err, errResolveHomeDir) {
		log.Println("Failed to get the user home directory")
		os.Exit(1)
	}

	log.Fatal(err)
}

var (
	errMissingPath    = errors.New("missing -path flag")
	errResolveHomeDir = errors.New("failed to resolve current user")
	errInvalidWAVFile = errors.New("invalid WAV file")
)

func run(args []string, currentUser func() (*user.User, error), out io.Writer) error {
	fs := flag.NewFlagSet("wavtoaiff", flag.ContinueOnError)

	pathFlag := fs.String("path", "", "The path to the wav file to convert to aiff")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *pathFlag == "" {
		return errMissingPath
	}

	usr, err := currentUser()
	if err != nil {
		return errResolveHomeDir
	}

	sourcePath := *pathFlag
	if strings.HasPrefix(sourcePath, "~/") {
		sourcePath = strings.Replace(sourcePath, "~", usr.HomeDir, 1)
	}

	file, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("invalid path %s: %w", sourcePath, err)
	}
	defer file.Close()

	decoder := wav.NewDecoder(file)
	if !decoder.IsValidFile() {
		return errInvalidWAVFile
	}

	outPath := sourcePath[:len(sourcePath)-len(filepath.Ext(sourcePath))] + ".aif"

	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", outPath, err)
	}
	defer outFile.Close()

	encoder := aiff.NewEncoder(outFile, int(decoder.SampleRate), int(decoder.BitDepth), int(decoder.NumChans))

	format := &audio.Format{
		NumChannels: int(decoder.NumChans),
		SampleRate:  int(decoder.SampleRate),
	}

	bufferSize := 1000000
	buf := &audio.Float32Buffer{Data: make([]float32, bufferSize), Format: format}

	var num int
	for err == nil {
		num, err = decoder.PCMBuffer(buf)
		if err != nil {
			break
		}

		if num == 0 {
			break
		}

		data := buf.Data
		if num != len(data) {
			data = data[:num]
		}

		intBuf := float32ToIntBuffer(data, format, int(decoder.BitDepth))

		err := encoder.Write(intBuf)
		if err != nil {
			return fmt.Errorf("failed to write AIFF data: %w", err)
		}
	}

	if err := encoder.Close(); err != nil {
		return fmt.Errorf("failed to close AIFF encoder: %w", err)
	}

	fmt.Fprintf(out, "Wav file converted to %s\n", outPath)

	return nil
}

func float32ToIntBuffer(data []float32, format *audio.Format, bitDepth int) *audio.IntBuffer {
	intBuf := &audio.IntBuffer{
		Format:         format,
		SourceBitDepth: bitDepth,
		Data:           make([]int, len(data)),
	}
	for i, v := range data {
		intBuf.Data[i] = float32ToPCMInt(v, bitDepth)
	}

	return intBuf
}

func float32ToPCMInt(value float32, bitDepth int) int {
	value = clampFloat32(value, -1, 1)

	switch bitDepth {
	case 8:
		return int(float32ToPCMUint8(value))
	case 16:
		return int(float32ToPCMInt32(value, 16))
	case 24:
		return int(float32ToPCMInt32(value, 24))
	case 32:
		return int(float32ToPCMInt32(value, 32))
	default:
		return 0
	}
}

func float32ToPCMUint8(value float32) uint8 {
	value = clampFloat32(value, -1, 1)

	scaled := int(math.Round(float64((value + 1.0) * 127.5)))
	if scaled < 0 {
		return 0
	}

	if scaled > 255 {
		return 255
	}

	return uint8(scaled)
}

func float32ToPCMInt32(value float32, bitDepth int) int32 {
	value = clampFloat32(value, -1, 1)

	switch bitDepth {
	case 16:
		return clampScaledPCM(value, 32768.0, 32767)
	case 24:
		return clampScaledPCM(value, 8388608.0, 8388607)
	case 32:
		return clampScaledPCM(value, 2147483648.0, 2147483647)
	default:
		return 0
	}
}

func clampScaledPCM(value float32, scale float64, maxVal int64) int32 {
	sample := min(int64(math.Round(float64(value)*scale)), maxVal)

	minVal := int64(-scale)
	if sample < minVal {
		sample = minVal
	}

	return int32(sample)
}

func clampFloat32(value, minVal, maxVal float32) float32 {
	if value < minVal {
		return minVal
	}

	if value > maxVal {
		return maxVal
	}

	return value
}
