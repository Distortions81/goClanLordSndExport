package main

import (
	"encoding/binary"
	"math"
	"os"
)

// sampleRate is the desired output sample rate. It defaults to 44100 but can
// be overridden by the -sampleRate flag in main.go.
var sampleRate = 44100

// The sounds themselves contain their original sample rate and bit depth in the
// sound header. Each sound may differ so the decoder must read these values
// before resampling to the desired output rate.

// indexTable and stepTable implement the IMA ADPCM decoding tables.
var indexTable = [16]int{-1, -1, -1, -1, 2, 4, 6, 8, -1, -1, -1, -1, 2, 4, 6, 8}
var stepTable = [89]int{
	7, 8, 9, 10, 11, 12, 13, 14, 16, 17,
	19, 21, 23, 25, 28, 31, 34, 37, 41, 45,
	50, 55, 60, 66, 73, 80, 88, 97, 107, 118,
	130, 143, 157, 173, 190, 209, 230, 253, 279, 307,
	337, 371, 408, 449, 494, 544, 598, 658, 724, 796,
	876, 963, 1060, 1166, 1282, 1411, 1552, 1707, 1878, 2066,
	2272, 2499, 2749, 3024, 3327, 3660, 4026, 4428, 4871, 5358,
	5894, 6484, 7132, 7845, 8630, 9493, 10442, 11487, 12635, 13899,
	15289, 16818, 18500, 20350, 22385, 24623, 27086, 29794, 32767,
}

// decodeSound converts the raw sample data into 16-bit PCM, handling several
// different encodings and resampling from the source sample rate to the
// requested output rate.
func decodeSound(raw []byte, format int16, srcRate int, bits int) []int16 {
	var pcm []int16

	switch {
	case format == 1:
		// assume format 1 is IMA ADPCM
		pcm = decodeIMA(raw)
	case bits == 16:
		pcm = expand16(raw)
	default:
		pcm = expand8(raw)
	}

	return resampleSinc(pcm, srcRate, sampleRate)
}

// parseSoundHeader extracts the sample rate, bit depth, and returns the raw
// sample data without the header. The header format follows Apple's Sound
// Manager structures.
func parseSoundHeader(raw []byte) ([]byte, int, int) {
	if len(raw) < 22 {
		return raw, 22050, 8
	}

	// All header variants encode the sample rate at the same offset.
	sr := int(binary.BigEndian.Uint32(raw[8:12]) >> 16)
	enc := raw[20]

	switch enc {
	case 0: // stdSH
		return raw[22:], sr, 8
	case 1: // extSH
		if len(raw) >= 64 {
			bits := int(binary.BigEndian.Uint16(raw[48:50]))
			return raw[64:], sr, bits
		}
	case 2: // cmpSH
		if len(raw) >= 64 {
			bits := int(binary.BigEndian.Uint16(raw[62:64]))
			return raw[64:], sr, bits
		}
	}

	return raw[22:], sr, 8
}

// expand8 converts unsigned 8-bit PCM to signed 16-bit PCM with a simple
// low-pass filter.
func expand8(raw []byte) []int16 {
	out := make([]int16, len(raw))
	var prev int16
	for i, b := range raw {
		sample := int16(int(b)-128) << 8
		out[i] = (sample + prev) / 2
		prev = sample
	}
	return out
}

// expand16 converts big-endian 16-bit PCM to signed 16-bit PCM.
func expand16(raw []byte) []int16 {
	out := make([]int16, len(raw)/2)
	for i := 0; i < len(out); i++ {
		out[i] = int16(binary.BigEndian.Uint16(raw[2*i:]))
	}
	return out
}

// decodeIMA decodes IMA ADPCM data into 16-bit PCM.
func decodeIMA(data []byte) []int16 {
	out := make([]int16, len(data)*2)
	predictor := 0
	index := 0
	step := stepTable[index]
	var pos int
	for _, b := range data {
		for shift := uint(0); shift <= 4; shift += 4 {
			nibble := int((b >> shift) & 0x0F)
			diff := step >> 3
			if nibble&4 != 0 {
				diff += step
			}
			if nibble&2 != 0 {
				diff += step >> 1
			}
			if nibble&1 != 0 {
				diff += step >> 2
			}
			if nibble&8 != 0 {
				predictor -= diff
			} else {
				predictor += diff
			}
			if predictor > 32767 {
				predictor = 32767
			}
			if predictor < -32768 {
				predictor = -32768
			}
			index += indexTable[nibble]
			if index < 0 {
				index = 0
			}
			if index > 88 {
				index = 88
			}
			step = stepTable[index]
			out[pos] = int16(predictor)
			pos++
		}
	}
	return out[:pos]
}

// resampleSinc resamples the input PCM using a windowed sinc filter.
func resampleSinc(in []int16, src, dst int) []int16 {
	if src == dst {
		cp := make([]int16, len(in))
		copy(cp, in)
		return cp
	}
	ratio := float64(dst) / float64(src)
	outLen := int(math.Round(float64(len(in)) * ratio))
	out := make([]int16, outLen)
	filterWidth := 8
	for i := 0; i < outLen; i++ {
		t := float64(i) / ratio
		idx := int(math.Floor(t))
		frac := t - float64(idx)
		var sum float64
		var norm float64
		for j := -filterWidth; j <= filterWidth; j++ {
			pos := idx + j
			if pos < 0 || pos >= len(in) {
				continue
			}
			x := float64(j) - frac
			w := sinc(x) * sinc(x/float64(filterWidth))
			sum += float64(in[pos]) * w
			norm += w
		}
		out[i] = int16(sum / norm)
	}
	return out
}

func sinc(x float64) float64 {
	if x == 0 {
		return 1
	}
	x *= math.Pi
	return math.Sin(x) / x
}

// writeWAV writes the provided samples to a mono 16-bit WAV file.
func writeWAV(filename string, samples []int16) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	numSamples := len(samples)
	dataLen := numSamples * 2

	var header [44]byte
	copy(header[0:], []byte("RIFF"))
	binary.LittleEndian.PutUint32(header[4:], uint32(36+dataLen))
	copy(header[8:], []byte("WAVE"))
	copy(header[12:], []byte("fmt "))
	binary.LittleEndian.PutUint32(header[16:], 16) // PCM chunk size
	binary.LittleEndian.PutUint16(header[20:], 1)  // Audio format PCM
	binary.LittleEndian.PutUint16(header[22:], 1)  // Mono
	binary.LittleEndian.PutUint32(header[24:], uint32(sampleRate))
	byteRate := sampleRate * 2 * 1
	binary.LittleEndian.PutUint32(header[28:], uint32(byteRate))
	blockAlign := 2 * 1
	binary.LittleEndian.PutUint16(header[32:], uint16(blockAlign))
	binary.LittleEndian.PutUint16(header[34:], 16) // Bits per sample
	copy(header[36:], []byte("data"))
	binary.LittleEndian.PutUint32(header[40:], uint32(dataLen))

	if _, err := f.Write(header[:]); err != nil {
		return err
	}

	return binary.Write(f, binary.LittleEndian, samples)
}
