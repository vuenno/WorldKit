// Package clipboard puts images on the Windows clipboard, through user32 with
// golang.org/x/sys/windows: no CGO. Gio's clipboard carries text only.
package clipboard

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                         = windows.NewLazySystemDLL("user32.dll")
	kernel32                       = windows.NewLazySystemDLL("kernel32.dll")
	procOpenClipboard              = user32.NewProc("OpenClipboard")
	procCloseClipboard             = user32.NewProc("CloseClipboard")
	procEmptyClipboard             = user32.NewProc("EmptyClipboard")
	procSetClipboardData           = user32.NewProc("SetClipboardData")
	procRegisterClipboardFormatW   = user32.NewProc("RegisterClipboardFormatW")
	procGlobalAlloc                = kernel32.NewProc("GlobalAlloc")
	procGlobalLock                 = kernel32.NewProc("GlobalLock")
	procGlobalUnlock               = kernel32.NewProc("GlobalUnlock")
	procGlobalFree                 = kernel32.NewProc("GlobalFree")
	errClipboardBusy               = errors.New("the clipboard is in use by another app")
	openAttempts, openAttemptPause = 10, 20 * time.Millisecond
)

const (
	formatDIBV5   = 17  // CF_DIBV5
	gmemMoveable  = 0x2 // GMEM_MOVEABLE: the clipboard needs movable memory
	pngFormatName = "PNG"
)

// CopyImage replaces the clipboard's contents with img, twice over: as PNG, which keeps
// transparency and is what browsers, Discord and image editors paste, and as a 32-bit
// bitmap with alpha (CF_DIBV5) for apps that only take bitmaps. It may block briefly
// while another app holds the clipboard, so call it off the UI goroutine.
func CopyImage(img image.Image) error {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		return fmt.Errorf("encode clipboard image: %w", err)
	}
	pngFormat, _, err := procRegisterClipboardFormatW.Call(uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(pngFormatName))))
	if pngFormat == 0 {
		return fmt.Errorf("register PNG clipboard format: %w", err)
	}
	// The clipboard belongs to the thread that opened it until it closes it.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := openClipboard(); err != nil {
		return err
	}
	defer procCloseClipboard.Call() // its result is ignored: nothing to do if closing fails
	if r, _, err := procEmptyClipboard.Call(); r == 0 {
		return fmt.Errorf("empty clipboard: %w", err)
	}
	if err := setData(uint32(pngFormat), encoded.Bytes()); err != nil {
		return fmt.Errorf("put PNG on clipboard: %w", err)
	}
	if err := setData(formatDIBV5, encodeDIBV5(img)); err != nil {
		return fmt.Errorf("put bitmap on clipboard: %w", err)
	}
	return nil
}

// openClipboard opens the clipboard, retrying for a moment while another app has it open.
func openClipboard() error {
	for attempt := 0; attempt < openAttempts; attempt++ {
		if r, _, _ := procOpenClipboard.Call(0); r != 0 {
			return nil
		}
		time.Sleep(openAttemptPause)
	}
	return errClipboardBusy
}

// setData hands a copy of data to the open clipboard in format. Once SetClipboardData
// succeeds, the clipboard owns the memory.
func setData(format uint32, data []byte) error {
	handle, _, err := procGlobalAlloc.Call(gmemMoveable, uintptr(len(data)))
	if handle == 0 {
		return fmt.Errorf("allocate %d bytes: %w", len(data), err)
	}
	if err := fillGlobal(handle, data); err != nil {
		procGlobalFree.Call(handle) // its result is ignored: already failing for the reason returned
		return err
	}
	if r, _, err := procSetClipboardData.Call(uintptr(format), handle); r == 0 {
		procGlobalFree.Call(handle) // its result is ignored: already failing for the reason returned
		return fmt.Errorf("set clipboard data: %w", err)
	}
	return nil
}

func fillGlobal(handle uintptr, data []byte) error {
	address, _, err := procGlobalLock.Call(handle)
	if address == 0 {
		return fmt.Errorf("lock memory: %w", err)
	}
	// Global memory isn't Go memory, so the address can be reinterpreted as a pointer.
	memory := *(*unsafe.Pointer)(unsafe.Pointer(&address))
	copy(unsafe.Slice((*byte)(memory), len(data)), data)
	procGlobalUnlock.Call(handle) // its result is ignored: 0 means unlocked, not a failure
	return nil
}
