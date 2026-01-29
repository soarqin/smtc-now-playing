package main

import (
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/windows"
)

var (
	libHandle uintptr

	// Function pointers
	smtcCreate            func() unsafe.Pointer
	smtcDestroy           func(smtc unsafe.Pointer)
	smtcInit              func(smtc unsafe.Pointer) int32
	smtcUpdate            func(smtc unsafe.Pointer)
	smtcRetrieveDirtyData func(smtc unsafe.Pointer, artist **uint16, title **uint16, thumbnailContentType **uint16, thumbnailData **uint8, thumbnailLength *int32, position *int32, duration *int32, status *int32) int32
)

// Global variables to hold C pointers
var (
	artist_c               *uint16
	title_c                *uint16
	thumbnailContentType_c *uint16
	thumbnailData_c        *uint8
	thumbnailLength_c      int32
)

func openLibrary(name string) (uintptr, error) {
	// Use syscall.LoadLibrary for Windows to avoid external dependencies
	handle, err := syscall.LoadLibrary(name)
	return uintptr(handle), err
}

func init() {
	// Load DLL
	if runtime.GOOS == "windows" {
		var err error
		libHandle, err = openLibrary("smtc.dll")
		if err != nil {
			// If all relative paths fail, try absolute path based on executable location
			var exePathBuf [260]uint16
			exePathLen, _ := windows.GetModuleFileName(0, &exePathBuf[0], uint32(len(exePathBuf)))
			if exePathLen > 0 {
				exePath := windows.UTF16ToString(exePathBuf[:exePathLen])
				exeDir := filepath.Dir(exePath)
				dllPath := filepath.Join(exeDir, "smtc.dll")
				libHandle, err = openLibrary(dllPath)
			}
			if err != nil {
				panic("Failed to load smtc.dll: " + err.Error())
			}
		}
	} else {
		panic("Unsupported platform: " + runtime.GOOS)
	}

	// Register functions
	purego.RegisterLibFunc(&smtcCreate, libHandle, "smtc_create")
	purego.RegisterLibFunc(&smtcDestroy, libHandle, "smtc_destroy")
	purego.RegisterLibFunc(&smtcInit, libHandle, "smtc_init")
	purego.RegisterLibFunc(&smtcUpdate, libHandle, "smtc_update")
	purego.RegisterLibFunc(&smtcRetrieveDirtyData, libHandle, "smtc_retrieve_dirty_data")
}

type Smtc struct {
	smtc unsafe.Pointer
}

func SmtcCreate() *Smtc {
	return &Smtc{smtc: smtcCreate()}
}

func (s *Smtc) Destroy() {
	smtcDestroy(s.smtc)
}

func (s *Smtc) Init() int {
	return int(smtcInit(s.smtc))
}

func (s *Smtc) Update() {
	smtcUpdate(s.smtc)
}

func (s *Smtc) RetrieveDirtyData(artist *string, title *string, thumbnailContentType *string, thumbnailData *[]byte, position *int, duration *int, status *int) int {
	var position_c int32
	var duration_c int32
	var status_c int32

	result := int(smtcRetrieveDirtyData(s.smtc, &artist_c, &title_c, &thumbnailContentType_c, &thumbnailData_c, &thumbnailLength_c, &position_c, &duration_c, &status_c))

	if result&1 != 0 {
		if artist_c != nil {
			*artist = windows.UTF16PtrToString(artist_c)
		}
		if title_c != nil {
			*title = windows.UTF16PtrToString(title_c)
		}
		if thumbnailContentType_c != nil {
			*thumbnailContentType = windows.UTF16PtrToString(thumbnailContentType_c)
		}
		if thumbnailData_c != nil && thumbnailLength_c > 0 {
			*thumbnailData = unsafe.Slice(thumbnailData_c, thumbnailLength_c)
		}
	}
	if result&2 != 0 {
		*position = int(position_c)
		*duration = int(duration_c)
		*status = int(status_c)
	}
	return result
}
