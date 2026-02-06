package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"

	"github.com/cwbudde/wav"
)

func main() {
	err := run(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	flagSet := flag.NewFlagSet("gen-sine", flag.ContinueOnError)

	output := flagSet.String("output", "output.wav", "filename to write to")
	frequency := flagSet.Float64("frequency", 440, "frequency in hertz to generate")
	length := flagSet.Float64("length", 5, "length in seconds of output file")

	err := flagSet.Parse(args)
	if err != nil {
		return err
	}

	log.Printf("generating a %f sec sine wav at %f hz", *length, *frequency)

	file, err := os.Create(*output)
	if err != nil {
		return fmt.Errorf("error creating %s: %w", *output, err)
	}
	defer file.Close()

	const sampleRate = 48000

	wavOut := wav.NewEncoder(file, sampleRate, 16, 1, 1)
	numSamples := int(sampleRate * *length)

	for i := range numSamples {
		fv := math.Sin(float64(i) / sampleRate * *frequency * 2 * math.Pi)

		v := float32(fv)

		err := wavOut.WriteFrame(v)
		if err != nil {
			return err
		}
	}

	return wavOut.Close()
}
