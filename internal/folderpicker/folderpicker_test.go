package folderpicker

import (
	"os"
	"runtime"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// TestDialogConfigures creates and configures the dialog without showing it, which checks
// the COM method offsets used before Show.
func TestDialogConfigures(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.CoInitializeEx(0, coinitApartment); err != nil && !isChangedMode(err) {
		t.Fatal(err)
	}
	defer windows.CoUninitialize()

	dialog, err := createDialog()
	if err != nil {
		t.Fatal(err)
	}
	defer release(dialog)
	if err := configure(dialog, "Pick a folder", os.TempDir()); err != nil {
		t.Fatal(err)
	}
	var options uint32
	if hr := call(dialog, methodGetOptions, uintptr(unsafe.Pointer(&options))); hr != 0 || options&fosPickFolders == 0 {
		t.Errorf("options = %#x (HRESULT %#x), want folder picking on", options, hr)
	}
}
