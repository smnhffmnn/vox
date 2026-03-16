//go:build tray

package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

var (
	iconIdle       = generateIcon(color.RGBA{180, 180, 180, 255}) // gray
	iconRecording  = generateIcon(color.RGBA{220, 50, 50, 255})   // red
	iconProcessing = generateIcon(color.RGBA{230, 160, 40, 255})  // orange
)

func generateIcon(c color.RGBA) []byte {
	const size = 22
	const center = float64(size) / 2
	const radius = float64(size)/2 - 1

	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			dx := float64(x) + 0.5 - center
			dy := float64(y) + 0.5 - center
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist <= radius {
				img.SetRGBA(x, y, c)
			}
		}
	}

	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}
