// Package assets embeds WorldKit's images and fonts so the binary is self-contained.
package assets

import (
	"bytes"
	"embed"
	"image"
	"image/png"
	"sync"
)

//go:embed logo.png
var logoPNG []byte

// Fonts holds the UI typefaces, Geist and Geist Mono (OFL-1.1, see fonts/OFL.txt), as
// fonts/*.ttf: one static file per weight.
//
//go:embed fonts/*.ttf
var Fonts embed.FS

// Logo returns the decoded 384×384 WorldKit logo with a transparent background.
var Logo = sync.OnceValue(func() image.Image {
	logo, err := png.Decode(bytes.NewReader(logoPNG))
	if err != nil {
		panic("embedded logo.png is invalid: " + err.Error())
	}
	return logo
})
