package filedialog

import (
	"os"
	"testing"
	"unsafe"
)

// TestDialogsConfigure creates and configures each dialog without showing it, which
// checks the COM method offsets used before Show.
func TestDialogsConfigure(t *testing.T) {
	err := withCOM(func() error {
		folder, err := createDialog(&clsidFileOpenDialog, &iidFileOpenDialog)
		if err != nil {
			return err
		}
		defer release(folder)
		if err := configureFolderPicker(folder, "Pick a folder", os.TempDir()); err != nil {
			return err
		}
		var options uint32
		if hr := call(folder, methodGetOptions, uintptr(unsafe.Pointer(&options))); hr != 0 || options&fosPickFolders == 0 {
			t.Errorf("options = %#x (HRESULT %#x), want folder picking on", options, hr)
		}

		save, err := createDialog(&clsidFileSaveDialog, &iidFileSaveDialog)
		if err != nil {
			return err
		}
		defer release(save)
		if err := addOptions(save, fosOverwritePrompt); err != nil {
			return err
		}
		if err := setFileTypes(save, []FileType{{Name: "PNG image", Pattern: "*.png"}}); err != nil {
			return err
		}
		setText(save, methodSetDefaultExtension, "png")
		setText(save, methodSetFileName, "test.png")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWithExtension(t *testing.T) {
	jpeg := []string{"jpg", "jpeg"}
	tests := map[string]string{
		`C:\a\shot`:      `C:\a\shot.jpg`,
		`C:\a\shot.JPEG`: `C:\a\shot.JPEG`,
		`C:\a\shot.png`:  `C:\a\shot.png.jpg`,
		"":               "",
	}
	for path, want := range tests {
		if got := withExtension(path, jpeg); got != want {
			t.Errorf("withExtension(%q) = %q, want %q", path, got, want)
		}
	}
}
