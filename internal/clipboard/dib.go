package clipboard

import (
	"encoding/binary"
	"image"
	"image/color"
)

// BITMAPV5HEADER values for a 32-bit sRGB bitmap with straight alpha.
const (
	dibHeaderSize  = 124
	biBitfields    = 3
	lcsSRGB        = 0x73524742 // 'sRGB'
	lcsGMImages    = 4          // LCS_GM_IMAGES: perceptual rendering intent
	maskRed        = 0x00FF0000
	maskGreen      = 0x0000FF00
	maskBlue       = 0x000000FF
	maskAlpha      = 0xFF000000
	dibBytesPerPix = 4
)

// encodeDIBV5 lays img out as CF_DIBV5 data: a BITMAPV5HEADER, then bottom-up rows of
// BGRA pixels with straight (not premultiplied) alpha.
func encodeDIBV5(img image.Image) []byte {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	pixelBytes := width * height * dibBytesPerPix
	data := make([]byte, dibHeaderSize+pixelBytes)
	writeDIBV5Header(data, width, height, pixelBytes)
	pixels := data[dibHeaderSize:]
	for y := 0; y < height; y++ {
		row := pixels[(height-1-y)*width*dibBytesPerPix:]
		for x := 0; x < width; x++ {
			c := color.NRGBAModel.Convert(img.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.NRGBA)
			copy(row[x*dibBytesPerPix:], []byte{c.B, c.G, c.R, c.A})
		}
	}
	return data
}

func writeDIBV5Header(data []byte, width, height, pixelBytes int) {
	le := binary.LittleEndian
	le.PutUint32(data[0:], dibHeaderSize)
	le.PutUint32(data[4:], uint32(width))
	le.PutUint32(data[8:], uint32(height)) // positive: rows run bottom-up
	le.PutUint16(data[12:], 1)             // planes
	le.PutUint16(data[14:], 32)            // bits per pixel
	le.PutUint32(data[16:], biBitfields)
	le.PutUint32(data[20:], uint32(pixelBytes))
	le.PutUint32(data[40:], maskRed)
	le.PutUint32(data[44:], maskGreen)
	le.PutUint32(data[48:], maskBlue)
	le.PutUint32(data[52:], maskAlpha)
	le.PutUint32(data[56:], lcsSRGB)
	le.PutUint32(data[108:], lcsGMImages)
}
