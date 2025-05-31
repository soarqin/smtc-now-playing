package main

import (
	"runtime"
	"syscall"
	"unsafe"

	"github.com/rodrigocfd/windigo/win"
	"github.com/rodrigocfd/windigo/win/co"
)

var (
	kernel32        = syscall.NewLazyDLL("kernel32.dll")
	procCreateMutex = kernel32.NewProc("CreateMutexW")
)

func CreateMutex(name string) (uintptr, error) {
	ptr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	ret, _, err := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(ptr)))
	switch int(err.(syscall.Errno)) {
	case 0:
		return ret, nil
	default:
		return ret, err
	}
}

func main() {
	runtime.LockOSThread() // important: Windows GUI is single-threaded

	g, err := NewProcessExitGroup()
	if err != nil {
		panic(err)
	}
	defer g.Dispose()

	mutex, err := CreateMutex("org.soardev.SmtcNowPlaying")
	nullHwnd := win.HWND(0)
	if err != nil {
		nullHwnd.MessageBox("Cannot run multiple instances of SmtcNowPlaying", "Error", co.MB_ICONERROR)
		return
	}
	defer syscall.CloseHandle(syscall.Handle(mutex))

	gui := NewGui(g)
	gui.wnd.RunAsMain()
	return
}
