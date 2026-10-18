// Package assets embeds WorldKit's images so the binary is self-contained.
package assets

import (
	"bytes"
	_ "embed"
	"image"
	"image/png"
	"sync"
)

//go:embed logo.png
var logoPNG []byte

// Logo returns the decoded 384×384 WorldKit logo with a transparent background.
var Logo = sync.OnceValue(func() image.Image {
	logo, err := png.Decode(bytes.NewReader(logoPNG))
	if err != nil {
		panic("embedded logo.png is invalid: " + err.Error())
	}
	return logo
})
