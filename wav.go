package wav

import (
	"errors"
	"math"
	"time"
)

// ErrPCMChunkNotFound indicates a bad audio file without data.
var ErrPCMChunkNotFound = errors.New("PCM Chunk not found in audio file")

func nullTermStr(b []byte) string {
	return string(b[:clen(b)])
}

func clen(num []byte) int {
	for i := range num {
		if num[i] == 0 {
			return i
		}
	}

	return len(num)
}

func bytesNumFromDuration(dur time.Duration, sampleRate, bitDepth int) int {
	k := bitDepth / 8
	return samplesNumFromDuration(dur, sampleRate) * k
}

func samplesNumFromDuration(dur time.Duration, sampleRate int) int {
	return int(math.Floor(float64(dur / sampleDuration(sampleRate))))
}

func sampleDuration(sampleRate int) time.Duration {
	if sampleRate == 0 {
		return 0
	}

	return time.Second / time.Duration(math.Abs(float64(sampleRate)))
}
