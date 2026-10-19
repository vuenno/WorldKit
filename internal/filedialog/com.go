// Package filedialog shows Windows' own Explorer-style dialogs: pick a folder, open
// files and Save As (IFileOpenDialog / IFileSaveDialog), through COM with
// golang.org/x/sys/windows: no CGO.
package filedialog

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ErrCanceled is returned when the user closes a dialog without choosing.
var ErrCanceled = errors.New("file dialog canceled")

var (
	ole32                           = windows.NewLazySystemDLL("ole32.dll")
	shell32                         = windows.NewLazySystemDLL("shell32.dll")
	procCoCreateInstance            = ole32.NewProc("CoCreateInstance")
	procSHCreateItemFromParsingName = shell32.NewProc("SHCreateItemFromParsingName")

	clsidFileOpenDialog = windows.GUID{Data1: 0xDC1C5A9C, Data2: 0xE88A, Data3: 0x4DDE, Data4: [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7}}
	iidFileOpenDialog   = windows.GUID{Data1: 0xD57C7288, Data2: 0xD4AD, Data3: 0x4768, Data4: [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60}}
	clsidFileSaveDialog = windows.GUID{Data1: 0xC0B4E2F3, Data2: 0xBA21, Data3: 0x4773, Data4: [8]byte{0x8D, 0xBA, 0x33, 0x5E, 0xC9, 0x46, 0xEB, 0x8B}}
	iidFileSaveDialog   = windows.GUID{Data1: 0x84BCCD23, Data2: 0x5FDE, Data3: 0x4CDB, Data4: [8]byte{0xAE, 0xA4, 0xAF, 0x64, 0xB8, 0x3D, 0x78, 0xAB}}
	iidShellItem        = windows.GUID{Data1: 0x43826D1E, Data2: 0xE718, Data3: 0x42EE, Data4: [8]byte{0xBC, 0x55, 0xA1, 0xE2, 0x61, 0xC3, 0x7B, 0xFE}}
)

// Method indexes in the COM vtables used here: IUnknown first, then IModalWindow and
// IFileDialog for both dialogs, IFileOpenDialog's own methods after those; IShellItem
// and IShellItemArray for results.
const (
	methodRelease             = 2
	methodShow                = 3
	methodSetFileTypes        = 4
	methodSetOptions          = 9
	methodGetOptions          = 10
	methodSetFolder           = 12
	methodSetFileName         = 15
	methodSetTitle            = 17
	methodGetResult           = 20
	methodSetDefaultExtension = 22
	methodGetResults          = 27 // IFileOpenDialog
	methodGetDisplayName      = 5  // IShellItem
	methodGetCount            = 7  // IShellItemArray
	methodGetItemAt           = 8  // IShellItemArray

	fosOverwritePrompt  = 0x2
	fosStrictFileTypes  = 0x4
	fosNoChangeDir      = 0x8
	fosPickFolders      = 0x20
	fosForceFileSystem  = 0x40
	fosAllowMultiselect = 0x200
	fosPathMustExist    = 0x800
	fosFileMustExist    = 0x1000

	sigdnFileSysPath   = 0x80058000
	clsctxInprocServer = 0x1
	coinitApartment    = 0x2
	hresultCanceled    = 0x800704C7 // HRESULT_FROM_WIN32(ERROR_CANCELLED)
	rpcEChangedMode    = 0x80010106 // COM already initialized differently on this thread
	sFalse             = 0x1        // COM was already initialized on this thread (a success)
)

// FileType is one entry of a dialog's file type list, e.g. {"Images", "*.png;*.jpg"}.
type FileType struct {
	Name    string
	Pattern string
}

// withCOM runs show on a locked OS thread with COM initialized: COM objects belong to the
// thread that created them, and the dialogs need a single-threaded apartment.
func withCOM(show func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := windows.CoInitializeEx(0, coinitApartment)
	switch {
	case err == nil, hasHRESULT(err, sFalse):
		// Initialized here (S_FALSE: already was): each success needs its own uninitialize.
		defer windows.CoUninitialize()
	case hasHRESULT(err, rpcEChangedMode):
		// Someone initialized this thread differently; that works, and it isn't ours to undo.
	default:
		return fmt.Errorf("initialize COM: %w", err)
	}
	return show()
}

// hasHRESULT reports whether err is the COM result code hr.
func hasHRESULT(err error, hr uint32) bool {
	var errno syscall.Errno
	return errors.As(err, &errno) && uint32(errno) == hr
}

func createDialog(clsid, iid *windows.GUID) (*comObject, error) {
	var dialog *comObject
	hr, _, _ := procCoCreateInstance.Call(uintptr(unsafe.Pointer(clsid)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&dialog)))
	if hr != 0 {
		return nil, fmt.Errorf("create file dialog: HRESULT %#x", hr)
	}
	return dialog, nil
}

// addOptions turns on the given FOS_ flags, keeping the dialog's defaults.
func addOptions(dialog *comObject, flags uint32) error {
	var options uint32
	if hr := call(dialog, methodGetOptions, uintptr(unsafe.Pointer(&options))); hr != 0 {
		return fmt.Errorf("read file dialog options: HRESULT %#x", hr)
	}
	if hr := call(dialog, methodSetOptions, uintptr(options|flags)); hr != 0 {
		return fmt.Errorf("set file dialog options: HRESULT %#x", hr)
	}
	return nil
}

// setText calls a method that takes one string; failures are harmless (a missing title
// or file name), so they are ignored.
func setText(dialog *comObject, method int, text string) {
	if text == "" {
		return
	}
	textPtr, err := windows.UTF16PtrFromString(text)
	if err != nil {
		return
	}
	call(dialog, method, uintptr(unsafe.Pointer(textPtr)))
}

// setFileTypes fills the dialog's file type list. Windows copies the strings.
func setFileTypes(dialog *comObject, types []FileType) error {
	if len(types) == 0 {
		return nil
	}
	type filterSpec struct{ name, pattern *uint16 }
	specs := make([]filterSpec, len(types))
	for i, t := range types {
		name, err := windows.UTF16PtrFromString(t.Name)
		if err != nil {
			return fmt.Errorf("file type %q: %w", t.Name, err)
		}
		pattern, err := windows.UTF16PtrFromString(t.Pattern)
		if err != nil {
			return fmt.Errorf("file type %q: %w", t.Pattern, err)
		}
		specs[i] = filterSpec{name, pattern}
	}
	if hr := call(dialog, methodSetFileTypes, uintptr(len(specs)), uintptr(unsafe.Pointer(&specs[0]))); hr != 0 {
		return fmt.Errorf("set file types: HRESULT %#x", hr)
	}
	return nil
}

// setStartFolder opens the dialog in dir; if dir doesn't exist the dialog opens where it
// last was, which is fine, so failures are ignored.
func setStartFolder(dialog *comObject, dir string) {
	if dir == "" {
		return
	}
	dirPtr, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return
	}
	var item *comObject
	hr, _, _ := procSHCreateItemFromParsingName.Call(uintptr(unsafe.Pointer(dirPtr)), 0,
		uintptr(unsafe.Pointer(&iidShellItem)), uintptr(unsafe.Pointer(&item)))
	if hr != 0 {
		return
	}
	defer release(item)
	call(dialog, methodSetFolder, uintptr(unsafe.Pointer(item)))
}

// show runs the dialog modally over owner.
func show(dialog *comObject, owner uintptr) error {
	hr := call(dialog, methodShow, owner)
	switch hr {
	case 0:
		return nil
	case hresultCanceled:
		return ErrCanceled
	}
	return fmt.Errorf("show file dialog: HRESULT %#x", hr)
}

// resultPath returns the path the user chose in a single-choice dialog.
func resultPath(dialog *comObject) (string, error) {
	var item *comObject
	if hr := call(dialog, methodGetResult, uintptr(unsafe.Pointer(&item))); hr != 0 {
		return "", fmt.Errorf("read chosen item: HRESULT %#x", hr)
	}
	defer release(item)
	return itemPath(item)
}

func itemPath(item *comObject) (string, error) {
	var pathPtr *uint16
	if hr := call(item, methodGetDisplayName, sigdnFileSysPath, uintptr(unsafe.Pointer(&pathPtr))); hr != 0 {
		return "", fmt.Errorf("read chosen path: HRESULT %#x", hr)
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(pathPtr))
	return windows.UTF16PtrToString(pathPtr), nil
}

// comObject is a COM interface pointer: its first field points at the method table.
type comObject struct {
	vtable *[maxMethods]uintptr
}

// maxMethods bounds the method tables indexed here (IFileOpenDialog's GetResults is 27).
const maxMethods = 32

// call invokes method number index on a COM object and returns its HRESULT. Arguments are
// often pointers converted to uintptr; uintptrescapes keeps what they point at alive and in
// place until the call returns, as it does for syscall.SyscallN itself.
//
//go:uintptrescapes
func call(object *comObject, index int, args ...uintptr) uintptr {
	hr, _, _ := syscall.SyscallN(object.vtable[index], append([]uintptr{uintptr(unsafe.Pointer(object))}, args...)...)
	return hr
}

func release(object *comObject) {
	call(object, methodRelease)
}
