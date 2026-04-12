//go:build !nogui

package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

var (
	trayIconIdle       = generateIcon(color.RGBA{140, 140, 140, 255}) // gray
	trayIconRecording  = generateIcon(color.RGBA{220, 50, 50, 255})   // red
	trayIconProcessing = generateIcon(color.RGBA{230, 160, 30, 255})  // orange
)

// Keep old names for desktop.go tray init
var iconIdle = trayIconIdle

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
		// Icon generation should never fail with programmatic images.
		// Log and return a minimal 1x1 transparent PNG as fallback.
		return []byte{
			0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
			0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
			0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00,
			0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x62, 0x00, 0x00, 0x00, 0x02,
			0x00, 0x01, 0xe5, 0x27, 0xde, 0xfc, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45,
			0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
		}
	}
	return buf.Bytes()
}
