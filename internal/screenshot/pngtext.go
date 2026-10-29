package screenshot

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/png"
	"strconv"
	"strings"
)

// PNG text chunks that record how a screenshot was drawn, so a viewer can find its rows
// (RowBoxes) without guessing. image/png neither writes nor reads text chunks, so they are
// spliced in and looked up here.
const (
	// lookKeyword records the chat look: "Arial 60" (font and chat scale).
	lookKeyword = "WorldKit:Chat"
	// fontSizeKeyword is what screenshots drawn before chat looks recorded: their font
	// size in pixels.
	fontSizeKeyword = "WorldKit:FontSize"
)

const (
	pngSignatureLength = 8
	chunkHeaderLength  = 8 // length + type
	chunkCRCLength     = 4
)

// encodePNG encodes img as a PNG with look recorded in a tEXt chunk right after IHDR.
func encodePNG(img image.Image, look Look) ([]byte, error) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		return nil, err
	}
	data := encoded.Bytes()
	// image/png always writes IHDR first, so the end of its first chunk is known to exist.
	afterHeader := pngSignatureLength + chunkHeaderLength + int(binary.BigEndian.Uint32(data[pngSignatureLength:])) + chunkCRCLength
	text := textChunk(lookKeyword, fmt.Sprintf("%s %d", look.Font, ClampChatScale(look.ChatScale)))
	return bytes.Join([][]byte{data[:afterHeader], text, data[afterHeader:]}, nil), nil
}

// textChunk builds a PNG tEXt chunk: keyword, a zero byte, then the text.
func textChunk(keyword, text string) []byte {
	body := append([]byte("tEXt"+keyword+"\x00"), text...)
	chunk := binary.BigEndian.AppendUint32(nil, uint32(len(body)-4))
	chunk = append(chunk, body...)
	return binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(body))
}

// savedGrid returns where the rows are in a saved screenshot's PNG data: from its chat
// look, or for older screenshots from their font size.
func savedGrid(data []byte) RowGrid {
	if text, ok := findText(data, lookKeyword); ok {
		if look, ok := parseLook(text); ok {
			return chatGrid(geometryFor(look))
		}
	}
	size := legacyDefaultFontSize // screenshots from before sizes could be picked
	if text, ok := findText(data, fontSizeKeyword); ok {
		if parsed, err := strconv.Atoi(text); err == nil {
			size = parsed
		}
	}
	return legacyGrid(size)
}

func parseLook(text string) (Look, bool) {
	name, scale, ok := strings.Cut(text, " ")
	if !ok {
		return Look{}, false
	}
	percent, err := strconv.Atoi(scale)
	if err != nil {
		return Look{}, false
	}
	return Look{Font: ParseChatFont(name), ChatScale: ClampChatScale(percent)}, true
}

// Screenshots drawn before chat looks: Arial Bold at 10 to 24 px (14 by default), with a
// hard 1 px outline, rows 18/14 of the font size tall, 7 px in from the edges.
const (
	legacyDefaultFontSize = 14
	legacyMinFontSize     = 10
	legacyMaxFontSize     = 24
	legacyLineHeightPx    = 18 // at legacyDefaultFontSize
	legacyMarginPx        = 7
	legacyOutlinePx       = 1
)

func legacyGrid(fontSize int) RowGrid {
	fontSize = max(legacyMinFontSize, min(fontSize, legacyMaxFontSize))
	lineHeight := (fontSize*legacyLineHeightPx + legacyDefaultFontSize/2) / legacyDefaultFontSize
	return RowGrid{Top: legacyMarginPx, Height: float64(lineHeight), Reach: legacyOutlinePx}
}

// findText returns the text of the tEXt chunk named keyword, looking at the chunks before
// the image data (where encodePNG puts it).
func findText(data []byte, keyword string) (string, bool) {
	for at := pngSignatureLength; at+chunkHeaderLength <= len(data); {
		length := int(binary.BigEndian.Uint32(data[at:]))
		kind := string(data[at+4 : at+chunkHeaderLength])
		bodyStart := at + chunkHeaderLength
		if kind == "IDAT" || length < 0 || bodyStart+length > len(data) {
			return "", false
		}
		if kind == "tEXt" {
			name, text, isText := bytes.Cut(data[bodyStart:bodyStart+length], []byte{0})
			if isText && string(name) == keyword {
				return string(text), true
			}
		}
		at = bodyStart + length + chunkCRCLength
	}
	return "", false
}
