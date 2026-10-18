package fivem

import (
	"encoding/binary"
	"errors"
	"fmt"
)

var errNotMsgpackString = errors.New("value is not a MessagePack string")

// decodeMsgpackString decodes a single MessagePack str value (fixstr, str8, str16 or str32).
// KVS chat values are always strings, so a full MessagePack decoder isn't needed.
func decodeMsgpackString(value []byte) (string, error) {
	if len(value) == 0 {
		return "", errNotMsgpackString
	}
	marker, rest := value[0], value[1:]
	var length int
	switch {
	case marker >= 0xa0 && marker <= 0xbf: // fixstr: length in the low 5 bits
		length = int(marker & 0x1f)
	case marker == 0xd9 && len(rest) >= 1: // str8
		length, rest = int(rest[0]), rest[1:]
	case marker == 0xda && len(rest) >= 2: // str16
		length, rest = int(binary.BigEndian.Uint16(rest)), rest[2:]
	case marker == 0xdb && len(rest) >= 4: // str32
		length, rest = int(binary.BigEndian.Uint32(rest)), rest[4:]
	default:
		return "", fmt.Errorf("marker 0x%02x: %w", marker, errNotMsgpackString)
	}
	if len(rest) < length {
		return "", fmt.Errorf("want %d bytes, have %d: %w", length, len(rest), errNotMsgpackString)
	}
	return string(rest[:length]), nil
}
