//go:build windows

package smtc

import (
	"syscall"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	winrt "github.com/saltosystems/winrt-go"
	"github.com/saltosystems/winrt-go/windows/foundation"
	"github.com/saltosystems/winrt-go/windows/storage/streams"
)

// WinRT interface IIDs for the thumbnail extraction pipeline.
// These are stable Windows SDK constants; not present in the winrt-go streams package.
const (
	// IRandomAccessStream — provides Size and GetInputStreamAt.
	// IID: {00000905-0000-0000-C000-000000000046}
	guidIRandomAccessStream = "00000905-0000-0000-c000-000000000046"

	// IContentTypeProvider — provides ContentType on IRandomAccessStreamWithContentType.
	// IID: {905A0FE6-BC53-11DF-8C49-001E4FC686DA}
	guidIContentTypeProvider = "905a0fe6-bc53-11df-8c49-001e4fc686da"

	// IInputStream — provides ReadAsync.
	// IID: {905A0FE2-BC53-11DF-8C49-001E4FC686DA}
	guidIInputStream = "905a0fe2-bc53-11df-8c49-001e4fc686da"

	// Signature of IRandomAccessStreamWithContentType for parameterized IID computation.
	// IID: {CC254827-4B3D-438F-9232-10C76BC7E038}
	sigIRandomAccessStreamWithContentType = "{cc254827-4b3d-438f-9232-10c76bc7e038}"
)

// iidOpenReadCompletedHandler is the parameterized IID for
// IAsyncOperationCompletedHandler<IRandomAccessStreamWithContentType>.
var iidOpenReadCompletedHandler *ole.GUID

func init() {
	iidOpenReadCompletedHandler = ole.NewGUID(winrt.ParameterizedInstanceGUID(
		foundation.GUIDAsyncOperationCompletedHandler,
		sigIRandomAccessStreamWithContentType,
	))
}

// --- Minimal COM vtable wrappers for WinRT interfaces not in winrt-go ---

// iRandomAccessStreamVtbl mirrors the vtable layout of IRandomAccessStream.
// Methods beyond getInputStreamAt are padding to preserve correct offsets.
type iRandomAccessStreamVtbl struct {
	ole.IInspectableVtbl         // [0-5]: QI, AddRef, Release, GetIids, GetRTC, GetTL
	getSize              uintptr // [6]: get_Size -> UINT64
	putSize              uintptr // [7]: put_Size <- UINT64 (unused)
	getInputStreamAt     uintptr // [8]: GetInputStreamAt(UINT64) -> IInputStream*
	// remaining methods unused
	_getOutputStreamAt uintptr // [9]
	_seek              uintptr // [10]
	_cloneStream       uintptr // [11]
	_getCanRead        uintptr // [12]
	_getCanWrite       uintptr // [13]
}

type iRandomAccessStream struct{ ole.IInspectable }

func (v *iRandomAccessStream) VTable() *iRandomAccessStreamVtbl {
	return (*iRandomAccessStreamVtbl)(unsafe.Pointer(v.RawVTable))
}

// getSize returns the stream size in bytes (IRandomAccessStream.get_Size).
func (v *iRandomAccessStream) getSize() (uint64, error) {
	var out uint64
	hr, _, _ := syscall.SyscallN(
		v.VTable().getSize,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(&out)),
	)
	if hr != 0 {
		return 0, ole.NewError(hr)
	}
	return out, nil
}

// getInputStreamAt returns an IInputStream at the given byte offset.
func (v *iRandomAccessStream) getInputStreamAt(position uint64) (unsafe.Pointer, error) {
	var out unsafe.Pointer
	hr, _, _ := syscall.SyscallN(
		v.VTable().getInputStreamAt,
		uintptr(unsafe.Pointer(v)),
		uintptr(position),
		uintptr(unsafe.Pointer(&out)),
	)
	if hr != 0 {
		return nil, ole.NewError(hr)
	}
	return out, nil
}

// iContentTypeProviderVtbl mirrors IContentTypeProvider vtable.
type iContentTypeProviderVtbl struct {
	ole.IInspectableVtbl         // [0-5]
	getContentType       uintptr // [6]: get_ContentType -> HSTRING
}

type iContentTypeProvider struct{ ole.IInspectable }

func (v *iContentTypeProvider) VTable() *iContentTypeProviderVtbl {
	return (*iContentTypeProviderVtbl)(unsafe.Pointer(v.RawVTable))
}

// getContentType returns the MIME content type of the stream.
func (v *iContentTypeProvider) getContentType() (string, error) {
	var outHStr ole.HString
	hr, _, _ := syscall.SyscallN(
		v.VTable().getContentType,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(&outHStr)),
	)
	if hr != 0 {
		return "", ole.NewError(hr)
	}
	result := outHStr.String()
	ole.DeleteHString(outHStr)
	return result, nil
}

// iInputStreamVtbl mirrors IInputStream vtable.
type iInputStreamVtbl struct {
	ole.IInspectableVtbl         // [0-5]
	readAsync            uintptr // [6]: ReadAsync(IBuffer*, UINT32, InputStreamOptions) -> IAsyncOperationWithProgress<IBuffer*, UINT32>*
}

type iInputStream struct{ ole.IInspectable }

func (v *iInputStream) VTable() *iInputStreamVtbl {
	return (*iInputStreamVtbl)(unsafe.Pointer(v.RawVTable))
}

// readAsync calls IInputStream.ReadAsync with InputStreamOptions.None (0).
// Returns a raw COM pointer to IAsyncOperationWithProgress<IBuffer, uint32>.
func (v *iInputStream) readAsync(buffer *streams.IBuffer, count uint32) (unsafe.Pointer, error) {
	var out unsafe.Pointer
	hr, _, _ := syscall.SyscallN(
		v.VTable().readAsync,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(buffer)),
		uintptr(count),
		0, // InputStreamOptions.None
		uintptr(unsafe.Pointer(&out)),
	)
	if hr != 0 {
		return nil, ole.NewError(hr)
	}
	return out, nil
}

// pollAsyncProgress waits for an IAsyncOperationWithProgress to complete
// by QI-ing the pointer for IAsyncInfo and polling GetStatus.
// This avoids needing to compute the parameterized handler IID for the progress type.
func pollAsyncProgress(opPtr unsafe.Pointer) foundation.AsyncStatus {
	if opPtr == nil {
		return foundation.AsyncStatusError
	}
	iunk := (*ole.IUnknown)(opPtr)
	itf, err := iunk.QueryInterface(ole.NewGUID(foundation.GUIDIAsyncInfo))
	if err != nil {
		return foundation.AsyncStatusError
	}
	defer itf.Release()
	info := (*foundation.IAsyncInfo)(unsafe.Pointer(itf))
	for {
		status, err := info.GetStatus()
		if err != nil {
			return foundation.AsyncStatusError
		}
		if status != foundation.AsyncStatusStarted {
			return status
		}
		time.Sleep(time.Millisecond)
	}
}

// iAsyncWithProgressVtbl is the vtable layout for IAsyncOperationWithProgress<IBuffer, uint32>.
//
//	[0-2]  IUnknown  (QI, AddRef, Release)
//	[3-5]  IInspectable (GetIids, GetRTC, GetTL)
//	[6]    put_Progress
//	[7]    get_Progress
//	[8]    put_Completed
//	[9]    get_Completed
//	[10]   GetResults  <- returns TResult (IBuffer*)
type iAsyncWithProgressVtbl struct {
	qi, addRef, release        uintptr // IUnknown [0-2]
	getIids, getRTC, getTL     uintptr // IInspectable [3-5]
	putProgress, getProgress   uintptr // [6-7]
	putCompleted, getCompleted uintptr // [8-9]
	getResults                 uintptr // [10]
}

// iAsyncWithProgress is a minimal COM struct for IAsyncOperationWithProgress<IBuffer, uint32>.
// The vtablePtr at offset 0 mirrors the COM object layout (first field is vtable pointer).
type iAsyncWithProgress struct {
	vtablePtr *iAsyncWithProgressVtbl
}

// getAsyncWithProgressResult extracts the IBuffer result from a completed
// IAsyncOperationWithProgress<IBuffer, uint32> via the typed vtable struct.
func getAsyncWithProgressResult(opPtr unsafe.Pointer) (unsafe.Pointer, error) {
	obj := (*iAsyncWithProgress)(opPtr)
	var result unsafe.Pointer
	hr, _, _ := syscall.SyscallN(
		obj.vtablePtr.getResults,
		uintptr(opPtr),
		uintptr(unsafe.Pointer(&result)),
	)
	if hr != 0 {
		return nil, ole.NewError(hr)
	}
	return result, nil
}

// readThumbnail reads thumbnail bytes from s.currentProperties.
// Returns (contentType, data). Returns ("", nil) on any error or when the
// thumbnail size is unchanged (size-based deduplication).
//
// Replicates C++ checkUpdateOfThumbnail() at c/smtc.cpp:263-302.
// Pipeline: GetThumbnail → OpenReadAsync → size dedup → ReadAsync → DataReaderFromBuffer → ReadBytes.
func (s *Smtc) readThumbnail() (contentType string, data []byte) {
	if s.currentProperties == nil {
		return "", nil
	}

	// Step 1: Get IRandomAccessStreamReference from media properties.
	thumbnail, err := s.currentProperties.GetThumbnail()
	if err != nil || thumbnail == nil {
		return "", nil
	}

	// Step 2: OpenReadAsync → IAsyncOperation<IRandomAccessStreamWithContentType>
	op, err := thumbnail.OpenReadAsync()
	if err != nil {
		return "", nil
	}
	result, status := waitForAsync(op, iidOpenReadCompletedHandler)
	if status != foundation.AsyncStatusCompleted || result == nil {
		return "", nil
	}

	// result is IRandomAccessStreamWithContentType* — QI for needed sub-interfaces.
	streamIUnk := (*ole.IUnknown)(result)

	// Step 3: Get stream size via IRandomAccessStream.get_Size.
	rasItf, err := streamIUnk.QueryInterface(ole.NewGUID(guidIRandomAccessStream))
	if err != nil {
		return "", nil
	}
	defer rasItf.Release()
	ras := (*iRandomAccessStream)(unsafe.Pointer(rasItf))
	size, err := ras.getSize()
	if err != nil || size == 0 {
		return "", nil
	}

	// Step 4: Size-based deduplication — skip if stream size is unchanged.
	// Replicates: if (currentThumbnailData_.size() == stream.Size()) return;
	if size == s.currentThumbnailSize && s.currentThumbnailSize > 0 {
		return "", nil
	}

	// Step 5: Get content type via IContentTypeProvider.
	ctItf, err := streamIUnk.QueryInterface(ole.NewGUID(guidIContentTypeProvider))
	if err == nil {
		ct := (*iContentTypeProvider)(unsafe.Pointer(ctItf))
		contentType, _ = ct.getContentType()
		ctItf.Release()
	}

	// Step 6: Create IBuffer with sufficient capacity for the whole stream.
	buf, err := streams.BufferCreate(uint32(size))
	if err != nil {
		return "", nil
	}
	defer (*ole.IUnknown)(unsafe.Pointer(buf)).Release()

	// QI Buffer for the IBuffer interface required by ReadAsync and DataReaderFromBuffer.
	ibufItf, err := buf.QueryInterface(ole.NewGUID(streams.GUIDIBuffer))
	if err != nil {
		return "", nil
	}
	defer ibufItf.Release()
	ibuf := (*streams.IBuffer)(unsafe.Pointer(ibufItf))

	// Step 7: QI stream for IInputStream and call ReadAsync.
	inputItf, err := streamIUnk.QueryInterface(ole.NewGUID(guidIInputStream))
	if err != nil {
		return "", nil
	}
	defer inputItf.Release()
	inputStream := (*iInputStream)(unsafe.Pointer(inputItf))

	asyncOp, err := inputStream.readAsync(ibuf, uint32(size))
	if err != nil || asyncOp == nil {
		return "", nil
	}
	defer (*ole.IUnknown)(asyncOp).Release()

	// Step 8: Wait for ReadAsync to complete (polls IAsyncInfo.GetStatus).
	opStatus := pollAsyncProgress(asyncOp)
	if opStatus != foundation.AsyncStatusCompleted {
		return "", nil
	}

	// Step 9: Retrieve the filled IBuffer from the completed operation.
	resultPtr, err := getAsyncWithProgressResult(asyncOp)
	if err != nil || resultPtr == nil {
		return "", nil
	}
	resultBuf := (*streams.IBuffer)(resultPtr)
	defer (*ole.IUnknown)(unsafe.Pointer(resultBuf)).Release()

	// Step 10: Get the actual number of bytes written by ReadAsync.
	bytesRead, err := resultBuf.GetLength()
	if err != nil || bytesRead == 0 {
		return "", nil
	}

	// Step 11: Create DataReader from the filled buffer (no LoadAsync needed —
	// DataReaderFromBuffer creates a reader whose data is already available).
	reader, err := streams.DataReaderFromBuffer(resultBuf)
	if err != nil {
		return "", nil
	}
	defer reader.Release()

	// Step 12: Read all bytes from the DataReader's internal buffer.
	data, err = reader.ReadBytes(bytesRead)
	if err != nil {
		return "", nil
	}

	// Update dedup state with the successfully read size.
	s.currentThumbnailSize = size
	return contentType, data
}
