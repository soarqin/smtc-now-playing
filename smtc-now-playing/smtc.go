package main

/*
#cgo LDFLAGS: -L../build/lib -lsmtc_c
#include "../smtc-monitor/smtc_c.h"
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

var artist_c [256]C.wchar_t
var title_c [256]C.wchar_t
var thumbnailPath_c [1024]C.wchar_t

func (s *Smtc) RetrieveDirtyData(artist *string, title *string, thumbnailPath *string, position *int, duration *int, status *int) int {
	var position_c C.int
	var duration_c C.int
	var status_c C.int
	result := int(C.smtc_retrieve_dirty_data(s.smtc, &artist_c[0], &title_c[0], &thumbnailPath_c[0], &position_c, &duration_c, &status_c))
	if result&1 != 0 {
		*artist = windows.UTF16PtrToString((*uint16)(&artist_c[0]))
		*title = windows.UTF16PtrToString((*uint16)(&title_c[0]))
		*thumbnailPath = windows.UTF16PtrToString((*uint16)(&thumbnailPath_c[0]))
	}
	if result&2 != 0 {
		*position = int(position_c)
		*duration = int(duration_c)
		*status = int(status_c)
	}
	return result
}
