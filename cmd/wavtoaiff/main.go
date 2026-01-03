// This tool converts an aiff file into an identical wav file and stores
// it in the same folder as the source.
package main

import (
	"flag"
	"fmt"
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

var (
	flagPath = flag.String("path", "", "The path to the wav file to convert to aiff")
)

func main() {
	flag.Parse()
	if *flagPath == "" {
		fmt.Println("You must set the -path flag")
		os.Exit(1)
	}

	usr, err := user.Current()
	if err != nil {
		log.Println("Failed to get the user home directory")
		os.Exit(1)
	}

	sourcePath := *flagPath
	if sourcePath[:2] == "~/" {
		sourcePath = strings.Replace(sourcePath, "~", usr.HomeDir, 1)
	}

	f, err := os.Open(*flagPath)
	if err != nil {
		fmt.Println("Invalid path", *flagPath, err)
		os.Exit(1)
	}
	defer f.Close()

	d := wav.NewDecoder(f)
	if !d.IsValidFile() {
		fmt.Println("invalid WAV file")
		os.Exit(1)
	}

	outPath := sourcePath[:len(sourcePath)-len(filepath.Ext(sourcePath))] + ".aif"
	of, err := os.Create(outPath)
	if err != nil {
		fmt.Println("Failed to create", outPath)
		panic(err)
	}
	defer of.Close()

	e := aiff.NewEncoder(of, int(d.SampleRate), int(d.BitDepth), int(d.NumChans))

	format := &audio.Format{
		NumChannels: int(d.NumChans),
		SampleRate:  int(d.SampleRate),
	}

	bufferSize := 1000000
	buf := &audio.Float32Buffer{Data: make([]float32, bufferSize), Format: format}
	var n int
	for err == nil {
		n, err = d.PCMBuffer(buf)
		if err != nil {
			break
		}
		if n == 0 {
			break
		}
		data := buf.Data
		if n != len(data) {
			data = data[:n]
		}
		intBuf := float32ToIntBuffer(data, format, int(d.BitDepth))
		if err := e.Write(intBuf); err != nil {
			panic(err)
		}
	}

	if err := e.Close(); err != nil {
		panic(err)
	}
	fmt.Printf("Wav file converted to %s\n", outPath)
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

func clampScaledPCM(value float32, scale float64, max int64) int32 {
	sample := int64(math.Round(float64(value) * scale))
	if sample > max {
		sample = max
	}
	min := int64(-scale)
	if sample < min {
		sample = min
	}
	return int32(sample)
}

func clampFloat32(value, min, max float32) float32 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
