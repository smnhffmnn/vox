package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

var (
	iconIdle       = generateIcon(color.RGBA{140, 140, 140, 255}) // gray
	iconRecording  = generateIcon(color.RGBA{220, 50, 50, 255})   // red
	iconProcessing = generateIcon(color.RGBA{230, 160, 30, 255})  // orange
)

func generateIcon(fill color.RGBA) []byte {
	const size = 44
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	cx, cy := float64(size)/2, float64(size)/2
	r := float64(size) / 2.5

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) - cx + 0.5
			dy := float64(y) - cy + 0.5
			dist := dx*dx + dy*dy
			if dist <= r*r {
				img.SetRGBA(x, y, fill)
			}
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic("icon generation failed: " + err.Error())
	}
	return buf.Bytes()
}
