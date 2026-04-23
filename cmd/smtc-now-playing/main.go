//go:build windows

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sync/errgroup"

	"github.com/rodrigocfd/windigo/co"
	"github.com/rodrigocfd/windigo/win"
	"smtc-now-playing/internal/config"
	"smtc-now-playing/internal/gui"
	"smtc-now-playing/internal/server"
	"smtc-now-playing/internal/smtc"
	"smtc-now-playing/internal/version"
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

	var debug bool
	var headless bool
	flag.BoolVar(&debug, "debug", false, "enable debug logging")
	flag.BoolVar(&headless, "headless", false, "run without GUI; HTTP+WS server only")
	flag.Parse()

	logLevel := slog.LevelInfo
	if debug {
		logLevel = slog.LevelDebug
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	slog.SetDefault(slog.New(handler))

	cfgPath, err := config.ResolvePath()
	if err != nil {
		slog.Warn("config path resolution failed, using defaults", "err", err)
		cfgPath = ""
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		slog.Warn("failed to load config, using defaults", "err", err)
		cfg = config.DefaultConfig()
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
	// Wrap main logic so deferred CloseHandle runs even when runApp wants
	// to exit with a non-zero code — os.Exit would otherwise bypass defer.
	exitCode := runApp(cfg, headless)
	_ = syscall.CloseHandle(syscall.Handle(mutex))
	os.Exit(exitCode)
}

// runApp builds the appropriate mode (headless or GUI) and runs until done.
// Split out so main() can clean up the single-instance mutex before os.Exit.
func runApp(cfg *config.Config, headless bool) int {
	smtcSvc := smtc.New(smtc.Options{InitialDevice: cfg.SMTC.SelectedDevice})

	srv, err := server.New(cfg, smtcSvc)
	if err != nil {
		slog.Error("failed to create server", "err", err)
		return 1
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return smtcSvc.Run(gctx) })
	g.Go(func() error { return srv.Run(gctx) })

	if !headless {
		guiInst := gui.New(cfg, srv, smtcSvc, version.Version)
		g.Go(func() error { return guiInst.Run(gctx) })
	} else {
		slog.Info("headless mode", "port", cfg.Server.Port, "url", fmt.Sprintf("http://localhost:%d", cfg.Server.Port))
	}

	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("service exit", "err", err)
		return 1
	}
	return 0
}
