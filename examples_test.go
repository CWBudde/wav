package wav

import (
	"fmt"
	"log"
	"os"
)

func ExampleDecoder_Duration() {
	file, err := os.Open("fixtures/kick.wav")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	dur, err := NewDecoder(file).Duration()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s duration: %s\n", file.Name(), dur)
	// Output: fixtures/kick.wav duration: 204.172335ms
}

func ExampleDecoder_IsValidFile() {
	file, err := os.Open("fixtures/kick.wav")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	fmt.Printf("is this file valid: %t", NewDecoder(file).IsValidFile())
	// Output: is this file valid: true
}

func ExampleEncoder_Write() {
	file, err := os.Open("fixtures/kick.wav")
	if err != nil {
		panic(fmt.Sprintf("couldn't open audio file - %v", err))
	}

	// Decode the original audio file
	// and collect audio content and information.
	decoder := NewDecoder(file)

	buf, err := decoder.FullPCMBuffer()
	if err != nil {
		panic(err)
	}

	file.Close()
	fmt.Println("Old file ->", decoder)

	// Destination file
	out, err := os.Create("testOutput/kick.wav")
	if err != nil {
		panic(fmt.Sprintf("couldn't create output file - %v", err))
	}

	// setup the encoder and write all the frames
	encoder := NewEncoder(out,
		buf.Format.SampleRate,
		int(decoder.BitDepth),
		buf.Format.NumChannels,
		int(decoder.WavAudioFormat))
	if err = encoder.Write(buf); err != nil {
		panic(err)
	}
	// close the encoder to make sure the headers are properly
	// set and the data is flushed.
	if err = encoder.Close(); err != nil {
		panic(err)
	}

	out.Close()

	// reopen to confirm things worked well
	out, err = os.Open("testOutput/kick.wav")
	if err != nil {
		panic(err)
	}

	d2 := NewDecoder(out)
	d2.ReadInfo()
	fmt.Println("New file ->", d2)
	out.Close()
	os.Remove(out.Name())

	// Output:
	// Old file -> Format: WAVE - 1 channels @ 22050 / 16 bits - Duration: 0.204172 seconds
	// New file -> Format: WAVE - 1 channels @ 22050 / 16 bits - Duration: 0.204172 seconds
}

func ExampleDecoder_ReadMetadata() {
	file, err := os.Open("fixtures/listinfo.wav")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	decoder := NewDecoder(file)
	decoder.ReadMetadata()

	if err := decoder.Err(); err != nil {
		log.Fatal(err)
	}

	if decoder.Metadata == nil {
		fmt.Println("No metadata found")
		return
	}

	fmt.Printf("Artist: %s\n", decoder.Metadata.Artist)
	fmt.Printf("Title:  %s\n", decoder.Metadata.Title)
	fmt.Printf("Album:  %s\n", decoder.Metadata.Product)
	fmt.Printf("Track:  %s\n", decoder.Metadata.TrackNbr)

	// Output:
	// Artist: artist
	// Title:  track title
	// Album:  album title
	// Track:  42
}
