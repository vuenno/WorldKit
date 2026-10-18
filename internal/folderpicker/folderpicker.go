// Package folderpicker shows Windows' own "Select folder" dialog (the Explorer-style
// IFileOpenDialog in folder mode), through COM with golang.org/x/sys/windows: no CGO.
package folderpicker

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ErrCanceled is returned when the user closes the dialog without picking a folder.
var ErrCanceled = errors.New("folder picker canceled")

var (
	ole32                           = windows.NewLazySystemDLL("ole32.dll")
	shell32                         = windows.NewLazySystemDLL("shell32.dll")
	procCoCreateInstance            = ole32.NewProc("CoCreateInstance")
	procSHCreateItemFromParsingName = shell32.NewProc("SHCreateItemFromParsingName")

	clsidFileOpenDialog = windows.GUID{Data1: 0xDC1C5A9C, Data2: 0xE88A, Data3: 0x4DDE, Data4: [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7}}
	iidFileOpenDialog   = windows.GUID{Data1: 0xD57C7288, Data2: 0xD4AD, Data3: 0x4768, Data4: [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60}}
	iidShellItem        = windows.GUID{Data1: 0x43826D1E, Data2: 0xE718, Data3: 0x42EE, Data4: [8]byte{0xBC, 0x55, 0xA1, 0xE2, 0x61, 0xC3, 0x7B, 0xFE}}
)

// Method indexes in the COM vtables used here (IUnknown first, then IModalWindow,
// IFileDialog for the dialog; IShellItem for the result).
const (
	methodRelease        = 2
	methodShow           = 3
	methodSetOptions     = 9
	methodGetOptions     = 10
	methodSetFolder      = 12
	methodSetTitle       = 17
	methodGetResult      = 20
	methodGetDisplayName = 5

	fosPickFolders     = 0x20
	fosForceFileSystem = 0x40
	sigdnFileSysPath   = 0x80058000
	clsctxInprocServer = 0x1
	coinitApartment    = 0x2
	hresultCanceled    = 0x800704C7 // HRESULT_FROM_WIN32(ERROR_CANCELLED)
	rpcEChangedMode    = 0x80010106 // COM already initialized differently on this thread
)

// Pick shows the dialog over owner (a window handle; 0 for none), starting in startDir
// when it exists, and blocks until the user picks a folder or cancels. Run it off the
// UI goroutine.
func Pick(owner uintptr, title, startDir string) (string, error) {
	// COM objects belong to the thread that created them, and the dialog needs an STA.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.CoInitializeEx(0, coinitApartment); err != nil && !isChangedMode(err) {
		return "", fmt.Errorf("initialize COM: %w", err)
	}
	defer windows.CoUninitialize()

	dialog, err := createDialog()
	if err != nil {
		return "", err
	}
	defer release(dialog)
	if err := configure(dialog, title, startDir); err != nil {
		return "", err
	}
	if hr := call(dialog, methodShow, owner); hr != 0 {
		if hr == hresultCanceled {
			return "", ErrCanceled
		}
		return "", fmt.Errorf("show folder picker: HRESULT %#x", hr)
	}
	return result(dialog)
}

func isChangedMode(err error) bool {
	var errno syscall.Errno
	return errors.As(err, &errno) && uint32(errno) == rpcEChangedMode
}

func createDialog() (*comObject, error) {
	var dialog *comObject
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidFileOpenDialog)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidFileOpenDialog)), uintptr(unsafe.Pointer(&dialog)))
	if hr != 0 {
		return nil, fmt.Errorf("create folder picker: HRESULT %#x", hr)
	}
	return dialog, nil
}

func configure(dialog *comObject, title, startDir string) error {
	var options uint32
	if hr := call(dialog, methodGetOptions, uintptr(unsafe.Pointer(&options))); hr != 0 {
		return fmt.Errorf("read folder picker options: HRESULT %#x", hr)
	}
	if hr := call(dialog, methodSetOptions, uintptr(options|fosPickFolders|fosForceFileSystem)); hr != 0 {
		return fmt.Errorf("set folder picker options: HRESULT %#x", hr)
	}
	titlePtr, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return fmt.Errorf("folder picker title: %w", err)
	}
	call(dialog, methodSetTitle, uintptr(unsafe.Pointer(titlePtr))) // a missing title is harmless
	setStartFolder(dialog, startDir)
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

func result(dialog *comObject) (string, error) {
	var item *comObject
	if hr := call(dialog, methodGetResult, uintptr(unsafe.Pointer(&item))); hr != 0 {
		return "", fmt.Errorf("read picked folder: HRESULT %#x", hr)
	}
	defer release(item)
	var pathPtr *uint16
	if hr := call(item, methodGetDisplayName, sigdnFileSysPath, uintptr(unsafe.Pointer(&pathPtr))); hr != 0 {
		return "", fmt.Errorf("read picked folder path: HRESULT %#x", hr)
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(pathPtr))
	return windows.UTF16PtrToString(pathPtr), nil
}

// comObject is a COM interface pointer: its first field points at the method table.
type comObject struct {
	vtable *[maxMethods]uintptr
}

// maxMethods bounds the method tables indexed here (IFileDialog's GetResult is 20).
const maxMethods = 32

// call invokes method number index on a COM object and returns its HRESULT.
func call(object *comObject, index int, args ...uintptr) uintptr {
	hr, _, _ := syscall.SyscallN(object.vtable[index], append([]uintptr{uintptr(unsafe.Pointer(object))}, args...)...)
	return hr
}

func release(object *comObject) {
	call(object, methodRelease)
}

// Dialog is the Windows folder picker, for callers that take it as a dependency.
type Dialog struct{}

// Pick shows the dialog; see the package-level Pick.
func (Dialog) Pick(owner uintptr, title, startDir string) (string, error) {
	return Pick(owner, title, startDir)
}
