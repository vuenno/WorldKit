package chatlog

import (
	"errors"
	"fmt"
	"log/slog"

	"golang.org/x/sys/windows"
)

// ErrArchiveInUse means another WorldKit instance already has the archive open.
var ErrArchiveInUse = errors.New("chat archive is in use by another WorldKit")

// archiveLock is a file held open without sharing, so a second instance cannot open it.
// Windows closes the handle when the process exits, so a crash never leaves a stale lock.
type archiveLock struct {
	handle windows.Handle
}

func acquireLock(path string) (*archiveLock, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, fmt.Errorf("lock archive %q: %w", path, err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE,
		0, nil, windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		return nil, ErrArchiveInUse
	}
	if err != nil {
		return nil, fmt.Errorf("lock archive %q: %w", path, err)
	}
	return &archiveLock{handle: handle}, nil
}

func (l *archiveLock) release() {
	if err := windows.CloseHandle(l.handle); err != nil {
		slog.Warn("release chat archive lock", "error", err)
	}
}
