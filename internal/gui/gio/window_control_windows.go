//go:build windows

package giogui

import (
	"errors"
	"fmt"
	"sync"
	"syscall"
	"unsafe"

	gioapp "gioui.org/app"
	"golang.org/x/sys/windows"
)

// Win32 constants for ShowWindow.
const (
	winSWHide    = 0
	winSWShow    = 5
	winSWRestore = 9
)

var (
	user32DLL                 = windows.NewLazySystemDLL("user32.dll")
	procEnumWindows           = user32DLL.NewProc("EnumWindows")
	procFindWindowW           = user32DLL.NewProc("FindWindowW")
	procGetWindowThreadProcID = user32DLL.NewProc("GetWindowThreadProcessId")
	procIsWindow              = user32DLL.NewProc("IsWindow")
	procIsWindowVisible       = user32DLL.NewProc("IsWindowVisible")
	procShowWindow            = user32DLL.NewProc("ShowWindow")
	procSetForegroundWindow   = user32DLL.NewProc("SetForegroundWindow")
)

var (
	mainHWND     windows.HWND
	mainHWNDLock sync.Mutex
)

var (
	currentEnumPID   uint32
	currentEnumFound windows.HWND
)

// findMainWindowHandle locates the top-level window belonging to this process.
// It prefers a previously found handle as long as it is still valid, then tries
// to enumerate top-level windows by PID and visibility, and finally falls back
// to a title-based search.
func findMainWindowHandle() (windows.HWND, error) {
	mainHWNDLock.Lock()
	defer mainHWNDLock.Unlock()

	if mainHWND != 0 {
		if ret, _, _ := procIsWindow.Call(uintptr(mainHWND)); ret != 0 {
			return mainHWND, nil
		}
		mainHWND = 0
	}

	// Enumerate top-level windows and pick the first visible one owned by this
	// process. In a single-window GUI app this is sufficient.
	currentEnumPID = windows.GetCurrentProcessId()
	currentEnumFound = 0

	callback := syscall.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		var winPID uint32
		procGetWindowThreadProcID.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&winPID)))
		if winPID != currentEnumPID {
			return 1 // continue
		}
		if visible, _, _ := procIsWindowVisible.Call(uintptr(hwnd)); visible == 0 {
			return 1 // continue
		}
		currentEnumFound = hwnd
		return 0 // stop enumeration
	})

	procEnumWindows.Call(callback, 0)
	if currentEnumFound != 0 {
		mainHWND = currentEnumFound
		return mainHWND, nil
	}

	// Fallback: search by window title. This helps when the process owns only
	// a single titled top-level window.
	if mainWindowTitle != "" {
		titlePtr, err := windows.UTF16PtrFromString(mainWindowTitle)
		if err == nil {
			if hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(titlePtr))); hwnd != 0 {
				mainHWND = windows.HWND(hwnd)
				return mainHWND, nil
			}
		}
	}

	return 0, fmt.Errorf("could not find main window for PID %d", currentEnumPID)
}

func windowStateSwitch(w *gioapp.Window, show bool) error {
	if w == nil {
		return errors.New("no window")
	}
	hwnd, err := findMainWindowHandle()
	if err != nil {
		return fmt.Errorf("switch state window: %w", err)
	}
	wstate := uintptr(winSWHide)
	if show {
		wstate = uintptr(winSWRestore)
	}
	procShowWindow.Call(uintptr(hwnd), wstate)
	if show {
		procSetForegroundWindow.Call(uintptr(hwnd))
	}
	return nil
}

// hideMainWindow hides the native window and removes it from the taskbar.
func hideMainWindow(w *gioapp.Window) error {
	return windowStateSwitch(w, false)
}

// showMainWindow restores and foregrounds the native window.
func showMainWindow(w *gioapp.Window) error {
	return windowStateSwitch(w, true)
}
