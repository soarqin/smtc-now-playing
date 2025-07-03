package main

/*
#cgo LDFLAGS: -L../build/lib -lsmtc_c
#include "../smtc_c/smtc_c.h"
*/
import "C"
import (
	"unsafe"

	"golang.org/x/sys/windows"
)

type Smtc struct {
	smtc unsafe.Pointer
}

func SmtcCreate() *Smtc {
	return &Smtc{smtc: C.smtc_create()}
}

func (s *Smtc) Destroy() {
	C.smtc_destroy(s.smtc)
}

func (s *Smtc) Init() int {
	return int(C.smtc_init(s.smtc))
}

func (s *Smtc) Update() {
	C.smtc_update(s.smtc)
}

var artist_c *C.wchar_t
var title_c *C.wchar_t
var thumbnailContentType_c *C.wchar_t
var thumbnailData_c *C.uint8_t
var thumbnailLength_c C.int

func (s *Smtc) RetrieveDirtyData(artist *string, title *string, thumbnailContentType *string, thumbnailData *[]byte, position *int, duration *int, status *int) int {
	var position_c C.int
	var duration_c C.int
	var status_c C.int
	result := int(C.smtc_retrieve_dirty_data(s.smtc, &artist_c, &title_c, &thumbnailContentType_c, &thumbnailData_c, &thumbnailLength_c, &position_c, &duration_c, &status_c))
	if result&1 != 0 {
		*artist = windows.UTF16PtrToString((*uint16)(artist_c))
		*title = windows.UTF16PtrToString((*uint16)(title_c))
		*thumbnailContentType = windows.UTF16PtrToString((*uint16)(thumbnailContentType_c))
		*thumbnailData = unsafe.Slice((*byte)(unsafe.Pointer(thumbnailData_c)), thumbnailLength_c)
	}
	if result&2 != 0 {
		*position = int(position_c)
		*duration = int(duration_c)
		*status = int(status_c)
	}
	return result
}
