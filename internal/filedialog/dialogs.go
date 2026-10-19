package filedialog

import (
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"
)

// PickFolder shows the "Select folder" dialog over owner (a window handle; 0 for none),
// starting in startDir when it exists, and blocks until the user picks a folder or
// cancels. Run it off the UI goroutine.
func PickFolder(owner uintptr, title, startDir string) (string, error) {
	var path string
	err := withCOM(func() error {
		dialog, err := createDialog(&clsidFileOpenDialog, &iidFileOpenDialog)
		if err != nil {
			return err
		}
		defer release(dialog)
		if err := configureFolderPicker(dialog, title, startDir); err != nil {
			return err
		}
		if err := show(dialog, owner); err != nil {
			return err
		}
		path, err = resultPath(dialog)
		return err
	})
	return path, err
}

func configureFolderPicker(dialog *comObject, title, startDir string) error {
	if err := addOptions(dialog, fosPickFolders|fosForceFileSystem); err != nil {
		return err
	}
	setText(dialog, methodSetTitle, title)
	setStartFolder(dialog, startDir)
	return nil
}

// OpenFiles shows the "Open" dialog with multiple selection and blocks until the user
// picks one or more files of types, or cancels.
func OpenFiles(owner uintptr, title, startDir string, types []FileType) ([]string, error) {
	var paths []string
	err := withCOM(func() error {
		dialog, err := createDialog(&clsidFileOpenDialog, &iidFileOpenDialog)
		if err != nil {
			return err
		}
		defer release(dialog)
		if err := addOptions(dialog, fosAllowMultiselect|fosFileMustExist|fosForceFileSystem|fosNoChangeDir); err != nil {
			return err
		}
		if err := setFileTypes(dialog, types); err != nil {
			return err
		}
		setText(dialog, methodSetTitle, title)
		setStartFolder(dialog, startDir)
		if err := show(dialog, owner); err != nil {
			return err
		}
		paths, err = resultPaths(dialog)
		return err
	})
	return paths, err
}

// resultPaths reads every file chosen in a multiple-selection dialog.
func resultPaths(dialog *comObject) ([]string, error) {
	var items *comObject
	if hr := call(dialog, methodGetResults, uintptr(unsafe.Pointer(&items))); hr != 0 {
		return nil, fmt.Errorf("read chosen files: HRESULT %#x", hr)
	}
	defer release(items)
	var count uint32
	if hr := call(items, methodGetCount, uintptr(unsafe.Pointer(&count))); hr != 0 {
		return nil, fmt.Errorf("count chosen files: HRESULT %#x", hr)
	}
	paths := make([]string, 0, count)
	for i := range count {
		var item *comObject
		if hr := call(items, methodGetItemAt, uintptr(i), uintptr(unsafe.Pointer(&item))); hr != 0 {
			return nil, fmt.Errorf("read chosen file %d: HRESULT %#x", i, hr)
		}
		path, err := itemPath(item)
		release(item)
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}

// SaveOptions set up the Save As dialog. It offers exactly one file type, so the file
// always gets the format the user chose before.
type SaveOptions struct {
	Title      string
	Folder     string   // where it opens
	FileName   string   // suggested name, with its extension
	Type       FileType // e.g. {"PNG image", "*.png"}
	Extensions []string // accepted extensions without the dot, the first being the default
}

// Save shows the Save As dialog and blocks until the user names a file or cancels.
// Windows asks before overwriting. The returned path always ends in one of the
// accepted extensions.
func Save(owner uintptr, options SaveOptions) (string, error) {
	var path string
	err := withCOM(func() error {
		dialog, err := createDialog(&clsidFileSaveDialog, &iidFileSaveDialog)
		if err != nil {
			return err
		}
		defer release(dialog)
		flags := uint32(fosOverwritePrompt | fosStrictFileTypes | fosForceFileSystem | fosPathMustExist | fosNoChangeDir)
		if err := addOptions(dialog, flags); err != nil {
			return err
		}
		if err := setFileTypes(dialog, []FileType{options.Type}); err != nil {
			return err
		}
		if len(options.Extensions) > 0 {
			setText(dialog, methodSetDefaultExtension, options.Extensions[0])
		}
		setText(dialog, methodSetTitle, options.Title)
		setText(dialog, methodSetFileName, options.FileName)
		setStartFolder(dialog, options.Folder)
		if err := show(dialog, owner); err != nil {
			return err
		}
		path, err = resultPath(dialog)
		return err
	})
	return withExtension(path, options.Extensions), err
}

// withExtension adds the default extension when path ends in none of the accepted ones.
func withExtension(path string, extensions []string) string {
	if path == "" || len(extensions) == 0 {
		return path
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	for _, accepted := range extensions {
		if ext == accepted {
			return path
		}
	}
	return path + "." + extensions[0]
}

// Dialog is the Windows file dialogs, for callers that take them as a dependency.
type Dialog struct{}

// PickFolder shows the folder dialog; see the package-level PickFolder.
func (Dialog) PickFolder(owner uintptr, title, startDir string) (string, error) {
	return PickFolder(owner, title, startDir)
}

// OpenFiles shows the multiple-selection Open dialog; see the package-level OpenFiles.
func (Dialog) OpenFiles(owner uintptr, title, startDir string, types []FileType) ([]string, error) {
	return OpenFiles(owner, title, startDir, types)
}

// Save shows the Save As dialog; see the package-level Save.
func (Dialog) Save(owner uintptr, options SaveOptions) (string, error) {
	return Save(owner, options)
}
