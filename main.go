//go:build !smtc_test

package main

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/rodrigocfd/windigo/co"
	"github.com/rodrigocfd/windigo/win"
	"smtc-now-playing/internal/config"
	"smtc-now-playing/internal/gui"
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
	runtime.LockOSThread()
	config.Load()

	mutex, err := CreateMutex("org.soardev.SmtcNowPlaying")
	nullHwnd := win.HWND(0)
	if err != nil {
		nullHwnd.MessageBox("Cannot run multiple instances of SmtcNowPlaying", "Error", co.MB_ICONERROR)
		return
	}
	defer syscall.CloseHandle(syscall.Handle(mutex))

	gui := gui.New()
	os.Exit(gui.Run())
}
