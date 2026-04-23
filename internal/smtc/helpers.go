//go:build windows

package smtc

import (
	"strings"
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/saltosystems/winrt-go/windows/foundation"
)

var (
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procCreateEventW        = kernel32.NewProc("CreateEventW")
	procSetEvent            = kernel32.NewProc("SetEvent")
	procWaitForSingleObject = kernel32.NewProc("WaitForSingleObject")
	procCloseHandle         = kernel32.NewProc("CloseHandle")

	combase            = syscall.NewLazyDLL("combase.dll")
	procRoUninitialize = combase.NewProc("RoUninitialize")
)

// asyncWaitTimeoutMillis caps how long waitForAsync will block on a single
// WinRT async operation. 5s is generous for local SMTC queries — if it
// takes longer than that SMTC is almost certainly stuck and we'd rather
// return a clean timeout than hang the dedicated goroutine forever.
const asyncWaitTimeoutMillis = 5000

const waitInfinite = 0xFFFFFFFF
const waitTimeoutResult = 0x00000102

// roUninitialize calls RoUninitialize from combase.dll — the proper counterpart to
// ole.RoInitialize. Must be called once for each successful RoInitialize call.
func roUninitialize() {
	procRoUninitialize.Call()
}

// createEvent creates an auto-reset, initially non-signaled Windows event object.
func createEvent() uintptr {
	h, _, _ := procCreateEventW.Call(0, 0, 0, 0)
	return h
}

// setEvent signals the Windows event object identified by h.
func setEvent(h uintptr) {
	procSetEvent.Call(h)
}

// waitForSingleObject blocks until the Windows event object h is signaled.
func waitForSingleObject(h uintptr) {
	procWaitForSingleObject.Call(h, waitInfinite)
}

// waitForSingleObjectTimeout blocks up to ms milliseconds for h. Returns true
// if h was signaled before the deadline, false on timeout.
func waitForSingleObjectTimeout(h uintptr, ms uint32) bool {
	ret, _, _ := procWaitForSingleObject.Call(h, uintptr(ms))
	return ret != waitTimeoutResult
}

// closeHandle closes the Windows event object handle h.
func closeHandle(h uintptr) {
	procCloseHandle.Call(h)
}

// asyncOperationStatus returns the current status of op by querying IAsyncInfo.
func asyncOperationStatus(op *foundation.IAsyncOperation) foundation.AsyncStatus {
	itf := op.MustQueryInterface(ole.NewGUID(foundation.GUIDIAsyncInfo))
	defer itf.Release()
	info := (*foundation.IAsyncInfo)(unsafe.Pointer(itf))
	status, _ := info.GetStatus()
	return status
}

// waitForAsync waits for a WinRT IAsyncOperation to complete using a Windows event object.
// handlerIID must be the IAsyncOperationCompletedHandler<T> IID matching op's result type.
// Returns the raw result pointer (caller must cast) and the final AsyncStatus.
// Replicates WaitForAsyncOperation<T> from c/smtc.cpp:96-121.
//
// If SetCompleted() fails or the wait times out, the async operation is
// considered failed and we return (nil, AsyncStatusError) rather than
// blocking the dedicated SMTC goroutine indefinitely.
func waitForAsync(op *foundation.IAsyncOperation, handlerIID *ole.GUID) (unsafe.Pointer, foundation.AsyncStatus) {
	if asyncOperationStatus(op) != foundation.AsyncStatusCompleted {
		hEvent := createEvent()
	if hEvent == 0 {
		log.Warn("waitForAsync: CreateEventW returned NULL")
		return nil, foundation.AsyncStatusError
	}
		handler := foundation.NewAsyncOperationCompletedHandler(handlerIID, func(
			_ *foundation.AsyncOperationCompletedHandler,
			_ *foundation.IAsyncOperation,
			_ foundation.AsyncStatus,
		) {
			setEvent(hEvent)
		})
		if err := op.SetCompleted(handler); err != nil {
			// Registration failed — nobody will ever signal hEvent, so
			// bail out instead of blocking forever in waitForSingleObject.
			log.Debug("waitForAsync: SetCompleted failed", "err", err)
			handler.Release()
			closeHandle(hEvent)
			return nil, foundation.AsyncStatusError
		}
		signaled := waitForSingleObjectTimeout(hEvent, asyncWaitTimeoutMillis)
		closeHandle(hEvent)
		handler.Release()
		if !signaled {
			log.Warn("waitForAsync: timed out waiting for completion")
			return nil, foundation.AsyncStatusError
		}
	}
	status := asyncOperationStatus(op)
	if status == foundation.AsyncStatusCompleted {
		result, _ := op.GetResults()
		return result, status
	}
	return nil, status
}

// readNullableBool safely reads a nullable WinRT bool reference.
// Returns false, false if ref is nil (not present).
// Returns value, true if present and successfully read.
func readNullableBool(ref *foundation.IReference) (bool, bool) {
	if ref == nil {
		return false, false
	}
	ptr, err := ref.GetValue()
	if err != nil {
		return false, false
	}
	// IReference<Boolean>.GetValue() stores the bool in the pointer itself.
	// Reinterpret the storage as bool.
	val := *(*bool)(unsafe.Pointer(&ptr))
	return val, true
}

// readNullableFloat64 safely reads a nullable WinRT float64 reference.
// Returns 0.0, false if ref is nil (not present).
// Returns value, true if present and successfully read.
func readNullableFloat64(ref *foundation.IReference) (float64, bool) {
	if ref == nil {
		return 0.0, false
	}
	ptr, err := ref.GetValue()
	if err != nil {
		return 0.0, false
	}
	// IReference<Double>.GetValue() stores the float64 bits in the pointer itself.
	// Reinterpret the storage as float64.
	val := *(*float64)(unsafe.Pointer(&ptr))
	return val, true
}

// readNullableInt32 safely reads a nullable WinRT int32 reference.
// Returns 0, false if ref is nil (not present).
// Returns value, true if present and successfully read.
func readNullableInt32(ref *foundation.IReference) (int32, bool) {
	if ref == nil {
		return 0, false
	}
	ptr, err := ref.GetValue()
	if err != nil {
		return 0, false
	}
	// IReference<Int32>.GetValue() stores the int32 in the pointer itself.
	// Reinterpret the storage as int32.
	val := *(*int32)(unsafe.Pointer(&ptr))
	return val, true
}

// friendlyAppName extracts a user-friendly application name from an appUserModelId.
// For UWP apps (format: "PackageFamilyName!AppId"), extracts the part after the last "!".
// For Win32 apps (format: "app.exe"), strips the ".exe" suffix (case-insensitive).
// Returns the result with the first letter uppercased, or the original input if empty.
func friendlyAppName(appUserModelId string) string {
	name := appUserModelId
	// UWP apps: extract part after last "!"
	if idx := strings.LastIndex(name, "!"); idx >= 0 {
		name = name[idx+1:]
	} else if strings.HasSuffix(strings.ToLower(name), ".exe") {
		// Win32 apps: strip ".exe" suffix (case-insensitive)
		name = name[:len(name)-4]
	}
	// Title-case: uppercase first letter
	if len(name) > 0 {
		name = strings.ToUpper(name[:1]) + name[1:]
	}
	// Fallback: if result is empty, return original
	if name == "" {
		return appUserModelId
	}
	return name
}
