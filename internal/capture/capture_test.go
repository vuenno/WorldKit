package capture

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
)

func TestServiceSavesChatLogFromDataDir(t *testing.T) {
	kvsDir := filepath.Join(t.TempDir(), "kvs")
	writeStore(t, kvsDir, map[string]string{
		"res:client:gtaw_server_version": "GTA World",
		"res:client:gtaw_chat_log":       "[DATE: 18/OCT/2026 | TIME: 09:57:01]\n[09:57:01] Welcome to GTA World.",
	})
	archiveDir := t.TempDir()

	changes := make(chan struct{}, 16)
	service := New(Options{
		ArchiveDir: archiveDir,
		DataDir:    kvsDir,
		OnChange:   func() { changes <- struct{}{} },
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		service.Run(ctx)
		close(done)
	}()
	defer func() {
		cancel()
		<-done
	}()

	waitFor(t, changes, func() bool { return service.Status().LinesSaved == 2 })

	saved, err := os.ReadFile(filepath.Join(archiveDir, "2026", "10", "2026-10-18_095701.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := "[DATE: 18/OCT/2026 | TIME: 09:57:01]\n[09:57:01] Welcome to GTA World.\n"
	if string(saved) != want {
		t.Errorf("saved %q, want %q", saved, want)
	}
	if status := service.Status(); status.State != StateWatching || status.DataDir != kvsDir {
		t.Errorf("status = %+v, want watching %q", status, kvsDir)
	}
}

func TestSetDataDirRejectsFolderWithoutFiveMData(t *testing.T) {
	service := New(Options{ArchiveDir: t.TempDir()})
	if _, err := service.SetDataDir(t.TempDir()); err == nil {
		t.Fatal("SetDataDir accepted an empty folder")
	}
}

func waitFor(t *testing.T, changes <-chan struct{}, isDone func() bool) {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for !isDone() {
		select {
		case <-changes:
		case <-timeout:
			t.Fatal("timed out waiting for capture")
		}
	}
}

func writeStore(t *testing.T, dir string, values map[string]string) {
	t.Helper()
	db, err := leveldb.OpenFile(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for key, value := range values {
		encoded := append(binary.BigEndian.AppendUint32([]byte{0xdb}, uint32(len(value))), value...)
		if err := db.Put([]byte(key), encoded, nil); err != nil {
			t.Fatal(err)
		}
	}
}
