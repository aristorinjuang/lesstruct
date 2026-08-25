package media

import (
	"image"
	"io"
	"math/bits"

	"golang.org/x/image/draw"
)

// phashSide is the edge length of the thumbnail a perceptual hash is computed
// from; the resulting hash carries phashSide² = 64 bits.
const phashSide = 8

// MaxPerceptualDistance is the largest Hamming distance (out of 64 bits) at
// which two perceptual hashes still describe the same picture — roughly 10%,
// conservative enough that distinct photographs stay apart while resizes,
// recompressions, and format conversions of one picture compare equal.
const MaxPerceptualDistance = 6

// PerceptualHash computes a 64-bit average hash (aHash) of the image: decode,
// downscale to 8×8, threshold each pixel's luma against the thumbnail mean.
// Visually identical pictures hash (nearly) identically regardless of their
// file bytes, resolution, or encoding, so callers can detect duplicate imagery
// that SHA-256 content dedup cannot see. Flat single-colour images all hash
// toward the same value — an accepted blind spot for cover detection.
func PerceptualHash(r io.Reader) (uint64, error) {
	img, err := decodeImage(r)
	if err != nil {
		return 0, err
	}

	scaled := image.NewRGBA(image.Rect(0, 0, phashSide, phashSide))
	draw.BiLinear.Scale(scaled, scaled.Bounds(), img, img.Bounds(), draw.Src, nil)

	lumas := make([]int, phashSide*phashSide)
	sum := 0
	for y := range phashSide {
		for x := range phashSide {
			r, g, b, _ := scaled.At(x, y).RGBA()
			luma := int((299*(r>>8) + 587*(g>>8) + 114*(b>>8)) / 1000)
			lumas[y*phashSide+x] = luma
			sum += luma
		}
	}
	mean := sum / len(lumas)

	var hash uint64
	for i, luma := range lumas {
		if luma >= mean {
			hash |= 1 << i
		}
	}
	return hash, nil
}

// HammingDistance counts the differing bits between two perceptual hashes.
func HammingDistance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

// PerceptuallySimilar reports whether two perceptual hashes describe the same
// picture — their Hamming distance is at most MaxPerceptualDistance.
func PerceptuallySimilar(a, b uint64) bool {
	return HammingDistance(a, b) <= MaxPerceptualDistance
}
