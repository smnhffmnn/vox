//go:build tray

package tray

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"math"
)

var (
	iconIdle       = generateIcon(180, 180, 180, 255) // gray
	iconRecording  = generateIcon(220, 50, 50, 255)   // red
	iconProcessing = generateIcon(230, 160, 40, 255)  // orange
)

// generateIcon creates a minimal 16x16 PNG with a filled circle.
func generateIcon(r, g, b, a uint8) []byte {
	const size = 16
	var buf bytes.Buffer

	// PNG signature
	buf.Write([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})

	// IHDR chunk
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:4], size)  // width
	binary.BigEndian.PutUint32(ihdr[4:8], size)  // height
	ihdr[8] = 8                                   // bit depth
	ihdr[9] = 6                                   // color type: RGBA
	ihdr[10] = 0                                  // compression
	ihdr[11] = 0                                  // filter
	ihdr[12] = 0                                  // interlace
	writeChunk(&buf, "IHDR", ihdr)

	// IDAT chunk — raw pixel data
	var rawData bytes.Buffer
	cx, cy := float64(size)/2, float64(size)/2
	radius := float64(size)/2 - 0.5

	for y := 0; y < size; y++ {
		rawData.WriteByte(0) // filter: none
		for x := 0; x < size; x++ {
			dist := math.Sqrt(math.Pow(float64(x)+0.5-cx, 2) + math.Pow(float64(y)+0.5-cy, 2))
			if dist <= radius {
				rawData.Write([]byte{r, g, b, a})
			} else if dist <= radius+1.0 {
				// Anti-alias edge
				alpha := uint8(float64(a) * (1 - (dist - radius)))
				rawData.Write([]byte{r, g, b, alpha})
			} else {
				rawData.Write([]byte{0, 0, 0, 0})
			}
		}
	}

	var compressed bytes.Buffer
	w, _ := zlib.NewWriterLevel(&compressed, zlib.BestCompression)
	w.Write(rawData.Bytes())
	w.Close()
	writeChunk(&buf, "IDAT", compressed.Bytes())

	// IEND chunk
	writeChunk(&buf, "IEND", nil)

	return buf.Bytes()
}

func writeChunk(buf *bytes.Buffer, chunkType string, data []byte) {
	// Length
	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(data)))
	buf.Write(length)

	// Type + Data for CRC
	typeAndData := make([]byte, 4+len(data))
	copy(typeAndData[0:4], chunkType)
	copy(typeAndData[4:], data)
	buf.Write(typeAndData)

	// CRC
	crc := make([]byte, 4)
	binary.BigEndian.PutUint32(crc, crc32.ChecksumIEEE(typeAndData))
	buf.Write(crc)
}
