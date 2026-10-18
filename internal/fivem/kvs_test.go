package fivem

import (
	"encoding/binary"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/syndtr/goleveldb/leveldb"
)

func TestDecodeMsgpackString(t *testing.T) {
	long := strings.Repeat("x", 300)
	tests := []struct {
		name    string
		value   []byte
		want    string
		wantErr bool
	}{
		{"fixstr", append([]byte{0xa5}, "hello"...), "hello", false},
		{"empty fixstr", []byte{0xa0}, "", false},
		{"str8", append([]byte{0xd9, 3}, "abc"...), "abc", false},
		{"str16", append([]byte{0xda, 0x01, 0x2c}, long...), long, false},
		{"str32", append([]byte{0xdb, 0, 0, 0, 2}, "hi"...), "hi", false},
		{"truncated", append([]byte{0xa5}, "hi"...), "", true},
		{"not a string", []byte{0x01}, "", true},
		{"empty", nil, "", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := decodeMsgpackString(test.value)
			if (err != nil) != test.wantErr || got != test.want {
				t.Errorf("got %q, %v; want %q, error=%v", got, err, test.want, test.wantErr)
			}
		})
	}
}

func TestReadChatLog(t *testing.T) {
	tests := []struct {
		name       string
		serverName string
		chatLog    string
		want       string
		wantErr    error
	}{
		{"GTA World log", "GTA World", "[09:57:01] Welcome to GTA World.", "[09:57:01] Welcome to GTA World.", nil},
		{"other server", "Some Other RP", "hello", "", ErrNoChatLog},
		{"no log yet", "GTA World", "", "", ErrNoChatLog},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			kvsDir := filepath.Join(t.TempDir(), "kvs")
			values := map[string]string{serverVersionKey: test.serverName}
			if test.chatLog != "" {
				values[chatLogKey] = test.chatLog
			}
			writeStore(t, kvsDir, values)

			got, err := ReadChatLog(kvsDir)
			if !errors.Is(err, test.wantErr) || got != test.want {
				t.Errorf("got %q, %v; want %q, %v", got, err, test.want, test.wantErr)
			}
		})
	}
}

func TestResolveKVSDir(t *testing.T) {
	citizenFX := t.TempDir()
	kvsDir := filepath.Join(citizenFX, "kvs")
	writeStore(t, kvsDir, map[string]string{serverVersionKey: "GTA World"})

	for _, input := range []string{citizenFX, kvsDir, `"` + kvsDir + `"`} {
		if got, err := ResolveKVSDir(input); err != nil || got != kvsDir {
			t.Errorf("ResolveKVSDir(%q) = %q, %v; want %q", input, got, err, kvsDir)
		}
	}
	if _, err := ResolveKVSDir(t.TempDir()); !errors.Is(err, ErrDataDirNotFound) {
		t.Errorf("empty folder error = %v, want ErrDataDirNotFound", err)
	}
}

// writeStore creates a LevelDB store laid out like FiveM's, with MessagePack string values.
func writeStore(t *testing.T, dir string, values map[string]string) {
	t.Helper()
	db, err := leveldb.OpenFile(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for key, value := range values {
		if err := db.Put([]byte(key), encodeMsgpackString(value), nil); err != nil {
			t.Fatal(err)
		}
	}
}

func encodeMsgpackString(value string) []byte {
	header := binary.BigEndian.AppendUint32([]byte{0xdb}, uint32(len(value)))
	return append(header, value...)
}
