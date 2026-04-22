//go:build !smtc_test

package main

import (
	"log/slog"
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

// Windows error code returned by CreateMutex when a mutex with the requested
// name already exists (and is therefore still owned by another process).
const errorAlreadyExists = 183

// CreateMutex creates (or opens) a named Windows mutex.
// Returns (handle, nil) iff the mutex was freshly created (i.e. we are the
// first instance). Returns (0, syscall.Errno(ERROR_ALREADY_EXISTS)) when
// another instance already owns the mutex — the freshly-opened handle is
// closed before returning so callers never have to worry about leaking it.
// Any other failure returns (0, err).
func CreateMutex(name string) (uintptr, error) {
	ptr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	ret, _, callErr := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(ptr)))
	// syscall.Syscall always returns a non-nil Errno; inspect its numeric
	// value. 0 == ERROR_SUCCESS: brand new mutex, we own single-instance.
	errno, _ := callErr.(syscall.Errno)
	if ret == 0 {
		// CreateMutex itself failed (invalid args, etc.).
		if errno == 0 {
			return 0, syscall.EINVAL
		}
		return 0, errno
	}
	if errno == errorAlreadyExists {
		// Another instance is running. Close the handle we just obtained
		// so we don't leak it and so the kernel refcount reflects reality.
		_ = syscall.CloseHandle(syscall.Handle(ret))
		return 0, errno
	}
	return ret, nil
}

func main() {
	runtime.LockOSThread()

	// Parse --debug flag
	debug := false
	for _, arg := range os.Args[1:] {
		if arg == "--debug" {
			debug = true
			break
		}
	}

	// Initialize slog with appropriate log level
	logLevel := slog.LevelInfo
	if debug {
		logLevel = slog.LevelDebug
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	slog.SetDefault(slog.New(handler))

	if err := config.Load(); err != nil {
		slog.Warn("failed to load config, using defaults", "err", err)
	}

	mutex, err := CreateMutex("org.soardev.SmtcNowPlaying")
	nullHwnd := win.HWND(0)
	if err != nil {
		if errno, ok := err.(syscall.Errno); ok && errno == errorAlreadyExists {
			nullHwnd.MessageBox(
				"Cannot run multiple instances of SmtcNowPlaying",
				"Error", co.MB_ICONERROR)
		} else {
			nullHwnd.MessageBox(
				"Failed to acquire single-instance mutex: "+err.Error(),
				"Error", co.MB_ICONERROR)
		}
		return
	}
	// Wrap main logic so deferred CloseHandle runs even when gui.Run wants
	// to exit with a non-zero code — os.Exit would otherwise bypass defer.
	exitCode := runApp()
	_ = syscall.CloseHandle(syscall.Handle(mutex))
	os.Exit(exitCode)
}

// runApp builds the GUI and runs the main message loop. Split out so main()
// can clean up the single-instance mutex *before* os.Exit is called.
func runApp() int {
	g := gui.New(Version)
	return g.Run()
}
