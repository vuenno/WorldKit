// Package fivem reads the GTA World data that the FiveM client keeps on the player's disk.
//
// FiveM stores script key-value pairs ("KVS") in a LevelDB database under
// %AppData%\CitizenFX\kvs. Keys are "res:<resource>:<key>" and values are MessagePack.
// GTA World's client resource is named "client".
package fivem

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

const (
	chatLogKey         = "res:client:gtaw_chat_log"
	serverVersionKey   = "res:client:gtaw_server_version"
	gtaWorldServerName = "GTA World"
)

var (
	// ErrDataDirNotFound means a folder does not contain FiveM's key-value store.
	ErrDataDirNotFound = errors.New("FiveM data folder not found")
	// ErrNoChatLog means the store holds no GTA World chat log (e.g. never joined the server).
	ErrNoChatLog = errors.New("no GTA World chat log in FiveM data")
)

// FindKVSDir returns FiveM's key-value store in its default location.
func FindKVSDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find FiveM data folder: %w", err)
	}
	return ResolveKVSDir(filepath.Join(configDir, "CitizenFX"))
}

// ResolveKVSDir accepts the CitizenFX folder or its kvs folder and returns the kvs folder.
func ResolveKVSDir(path string) (string, error) {
	path = filepath.Clean(strings.Trim(strings.TrimSpace(path), `"`))
	for _, dir := range []string{path, filepath.Join(path, "kvs")} {
		if isLevelDB(dir) {
			return dir, nil
		}
	}
	return "", fmt.Errorf("%q: %w", path, ErrDataDirNotFound)
}

func isLevelDB(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "CURRENT"))
	return err == nil && info.Mode().IsRegular()
}

// Stamp identifies one version of the store's files. It changes whenever FiveM writes.
type Stamp struct {
	LastModified time.Time
	TotalSize    int64
	FileCount    int
}

// ReadStamp reads the store's current Stamp without opening the database.
func ReadStamp(kvsDir string) (Stamp, error) {
	entries, err := os.ReadDir(kvsDir)
	if err != nil {
		return Stamp{}, fmt.Errorf("read FiveM data %q: %w", kvsDir, err)
	}
	var stamp Stamp
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue // removed while listing, or not a data file
		}
		stamp.FileCount++
		stamp.TotalSize += info.Size()
		if info.ModTime().After(stamp.LastModified) {
			stamp.LastModified = info.ModTime()
		}
	}
	return stamp, nil
}

// ReadChatLog returns the GTA World chat log text. It reads a temporary copy of the store,
// so the live database that FiveM holds open is never touched.
func ReadChatLog(kvsDir string) (string, error) {
	snapshotDir, err := os.MkdirTemp("", "worldkit-kvs-*")
	if err != nil {
		return "", fmt.Errorf("create FiveM data snapshot: %w", err)
	}
	defer removeSnapshot(snapshotDir)

	if err := copyStore(kvsDir, snapshotDir); err != nil {
		return "", err
	}
	return readChatLogFrom(snapshotDir)
}

func readChatLogFrom(dir string) (string, error) {
	db, err := leveldb.OpenFile(dir, &opt.Options{ReadOnly: true})
	if err != nil {
		return "", fmt.Errorf("open FiveM data snapshot: %w", err)
	}
	defer db.Close()

	// The store is shared by every FiveM server the player joins; only trust GTA World's.
	serverName, err := readString(db, serverVersionKey)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(serverName, gtaWorldServerName) {
		return "", fmt.Errorf("server %q: %w", serverName, ErrNoChatLog)
	}
	return readString(db, chatLogKey)
}

func readString(db *leveldb.DB, key string) (string, error) {
	value, err := db.Get([]byte(key), nil)
	if errors.Is(err, leveldb.ErrNotFound) {
		return "", fmt.Errorf("key %q: %w", key, ErrNoChatLog)
	}
	if err != nil {
		return "", fmt.Errorf("read key %q: %w", key, err)
	}
	text, err := decodeMsgpackString(value)
	if err != nil {
		return "", fmt.Errorf("decode key %q: %w", key, err)
	}
	return text, nil
}

// copyStore copies the database files. LOCK (held by FiveM) and LOG (diagnostics) are skipped.
func copyStore(from, to string) error {
	entries, err := os.ReadDir(from)
	if err != nil {
		return fmt.Errorf("read FiveM data %q: %w", from, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.Type().IsRegular() || name == "LOCK" || strings.HasPrefix(name, "LOG") {
			continue
		}
		if err := copyFile(filepath.Join(from, name), filepath.Join(to, name)); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(from, to string) error {
	source, err := os.Open(from)
	if err != nil {
		return fmt.Errorf("copy FiveM data file: %w", err)
	}
	defer source.Close()
	target, err := os.Create(to)
	if err != nil {
		return fmt.Errorf("copy FiveM data file: %w", err)
	}
	if _, err := io.Copy(target, source); err != nil {
		target.Close()
		return fmt.Errorf("copy FiveM data file %q: %w", from, err)
	}
	if err := target.Close(); err != nil {
		return fmt.Errorf("copy FiveM data file %q: %w", from, err)
	}
	return nil
}

func removeSnapshot(dir string) {
	if err := os.RemoveAll(dir); err != nil {
		slog.Warn("remove FiveM data snapshot", "path", dir, "error", err)
	}
}
