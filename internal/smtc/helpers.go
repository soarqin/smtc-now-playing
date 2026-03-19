//go:build windows

package smtc

import (
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

const waitInfinite = 0xFFFFFFFF

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
func waitForAsync(op *foundation.IAsyncOperation, handlerIID *ole.GUID) (unsafe.Pointer, foundation.AsyncStatus) {
	if asyncOperationStatus(op) != foundation.AsyncStatusCompleted {
		hEvent := createEvent()
		handler := foundation.NewAsyncOperationCompletedHandler(handlerIID, func(
			_ *foundation.AsyncOperationCompletedHandler,
			_ *foundation.IAsyncOperation,
			_ foundation.AsyncStatus,
		) {
			setEvent(hEvent)
		})
		_ = op.SetCompleted(handler)
		waitForSingleObject(hEvent)
		closeHandle(hEvent)
		handler.Release()
	}
	status := asyncOperationStatus(op)
	if status == foundation.AsyncStatusCompleted {
		result, _ := op.GetResults()
		return result, status
	}
	return nil, status
}
