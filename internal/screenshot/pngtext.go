package screenshot

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/png"
	"strconv"
)

// fontSizeKeyword names the PNG text chunk that records a screenshot's font size, so a
// viewer can find its rows (RowBoxes) without guessing. image/png neither writes nor
// reads text chunks, so they are spliced in and looked up here.
const fontSizeKeyword = "WorldKit:FontSize"

const (
	pngSignatureLength = 8
	chunkHeaderLength  = 8 // length + type
	chunkCRCLength     = 4
)

// encodePNG encodes img as a PNG with fontSize recorded in a tEXt chunk right after IHDR.
func encodePNG(img image.Image, fontSize int) ([]byte, error) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		return nil, err
	}
	data := encoded.Bytes()
	// image/png always writes IHDR first, so the end of its first chunk is known to exist.
	afterHeader := pngSignatureLength + chunkHeaderLength + int(binary.BigEndian.Uint32(data[pngSignatureLength:])) + chunkCRCLength
	text := textChunk(fontSizeKeyword, strconv.Itoa(fontSize))
	return bytes.Join([][]byte{data[:afterHeader], text, data[afterHeader:]}, nil), nil
}

// textChunk builds a PNG tEXt chunk: keyword, a zero byte, then the text.
func textChunk(keyword, text string) []byte {
	body := append([]byte("tEXt"+keyword+"\x00"), text...)
	chunk := binary.BigEndian.AppendUint32(nil, uint32(len(body)-4))
	chunk = append(chunk, body...)
	return binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(body))
}

// savedFontSize returns the font size recorded in PNG data, or DefaultFontSize when there
// is none (screenshots saved before sizes could be picked were all drawn at the default).
func savedFontSize(data []byte) int {
	text, ok := findText(data, fontSizeKeyword)
	if !ok {
		return DefaultFontSize
	}
	size, err := strconv.Atoi(text)
	if err != nil {
		return DefaultFontSize
	}
	return ClampFontSize(size)
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
