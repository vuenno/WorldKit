// Package atomicfile writes files so a crash never leaves a half-written one behind.
package atomicfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Write saves data to path through a temporary file in the same folder, synced and then
// renamed over path: readers see the old file or the new one, never part of it. The
// folder is created when missing.
func Write(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create folder %q: %w", dir, err)
	}
	temp, err := os.CreateTemp(dir, "saving-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary file in %q: %w", dir, err)
	}
	defer os.Remove(temp.Name()) // fails harmlessly once the rename has moved it
	_, writeErr := temp.Write(data)
	syncErr := temp.Sync()
	closeErr := temp.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		return fmt.Errorf("save %q: %w", path, err)
	}
	return nil
}
