package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
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
	mutex, err := CreateMutex("org.soardev.SmtcNowPlaying")
	nullHwnd := win.HWND(0)
	if err != nil {
		nullHwnd.MessageBox("Cannot run multiple instances of SmtcNowPlaying", "Error", co.MB_ICONERROR)
		return
	}
	defer syscall.CloseHandle(syscall.Handle(mutex))

	runtime.LockOSThread() // important: Windows GUI is single-threaded
	gui := NewGui()
	go func() {
		_, ok := <-gui.WindowCreated()
		if !ok {
			return
		}
		// Get current directory
		dir, err := os.Getwd()
		// Add .mod to PATHEXT
		os.Setenv("PATHEXT", os.Getenv("PATHEXT")+";.mod")
		monitor := NewMonitor(filepath.Join(dir, "SmtcMonitor"))
		if err != nil {
			nullHwnd.MessageBox(err.Error(), "Error", co.MB_ICONERROR)
			return
		}
		err = monitor.StartProcess()
		if err != nil {
			nullHwnd.MessageBox(err.Error(), "Error", co.MB_ICONERROR)
			return
		}
		monitorErrChan := monitor.GetErrorChannel()
		srv := NewWebServer("localhost", "11451", monitor)
		srvErrChan := srv.Error()
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt)
		signal.Notify(signalChan, syscall.SIGTERM)
		go func() {
			for {
				select {
				case err := <-monitorErrChan:
					fmt.Printf("Error: %v\n", err)
				case err := <-srvErrChan:
					fmt.Printf("Web server error: %v\n", err)
					srv.Stop()
					monitor.Stop()
					return
				case <-signalChan:
					fmt.Println("Received signal, stopping...")
					srv.Stop()
					monitor.Stop()
					return
				}
			}
		}()
		srv.Start()
		fmt.Printf("Server started at http://%s\n", srv.Address())
		monitor.Join()
	}()
	gui.wnd.RunAsMain()
	return
}
