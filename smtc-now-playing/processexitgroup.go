package main

// import (
// 	"os"
// 	"unsafe"

// 	"golang.org/x/sys/windows"
// )

// type ProcessExitGroup windows.Handle

// func NewProcessExitGroup() (ProcessExitGroup, error) {
// 	handle, err := windows.CreateJobObject(nil, nil)
// 	if err != nil {
// 		return 0, err
// 	}

// 	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
// 		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
// 			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
// 		},
// 	}
// 	if _, err := windows.SetInformationJobObject(
// 		handle,
// 		windows.JobObjectExtendedLimitInformation,
// 		uintptr(unsafe.Pointer(&info)),
// 		uint32(unsafe.Sizeof(info))); err != nil {
// 		return 0, err
// 	}

// 	return ProcessExitGroup(handle), nil
// }

// func (g ProcessExitGroup) Dispose() error {
// 	return windows.CloseHandle(windows.Handle(g))
// }

// func (g ProcessExitGroup) AddProcess(p *os.Process) error {
// 	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_TERMINATE|windows.PROCESS_SET_QUOTA, false, uint32(p.Pid))
// 	if err != nil {
// 		return err
// 	}
// 	defer windows.CloseHandle(handle)

// 	return windows.AssignProcessToJobObject(
// 		windows.Handle(g),
// 		handle)
// }
