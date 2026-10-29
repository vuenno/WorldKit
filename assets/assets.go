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

// MuktaRegular is Mukta Regular (OFL-1.1, see chatfonts/OFL.txt), one of the fonts GTA
// World's chat can be set to. Screenshots draw with it; the UI never does. The game loads
// only this weight from Google Fonts, so its bold is synthetic.
//
//go:embed chatfonts/Mukta-Regular.ttf
var MuktaRegular []byte

// Logo returns the decoded 384×384 WorldKit logo with a transparent background.
var Logo = sync.OnceValue(func() image.Image {
	logo, err := png.Decode(bytes.NewReader(logoPNG))
	if err != nil {
		panic("embedded logo.png is invalid: " + err.Error())
	}
	return logo
})
