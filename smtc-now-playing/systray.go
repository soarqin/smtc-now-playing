package main

import (
	"crypto/rand"
	"unsafe"

	"github.com/rodrigocfd/windigo/co"
	"github.com/rodrigocfd/windigo/win"
)

const NotifyIconMsg = co.WM_APP + 1

type NotifyIcon struct {
	hwnd win.HWND
	guid win.GUID
}

func NewNotifyIcon(hwnd win.HWND) (*NotifyIcon, error) {
	ni := &NotifyIcon{
		hwnd: hwnd,
		guid: newGUID(),
	}
	data := ni.newData()
	data.UFlags |= co.NIF_MESSAGE
	data.UCallbackMessage = NotifyIconMsg
	if err := win.Shell_NotifyIcon(co.NIM_ADD, data); err != nil {
		return nil, err
	}
	return ni, nil
}

func (ni *NotifyIcon) AddAfterExplorerCrash() {
	win.Shell_NotifyIcon(co.NIM_ADD, ni.newData())
}

func (ni *NotifyIcon) Dispose() {
	win.Shell_NotifyIcon(co.NIM_DELETE, ni.newData())
}

func (ni *NotifyIcon) SetTooltip(tooltip string) error {
	data := ni.newData()
	data.UFlags |= co.NIF_TIP
	data.SetSzTip(tooltip)
	if err := win.Shell_NotifyIcon(co.NIM_MODIFY, data); err != nil {
		return err
	}
	return nil
}

func (ni *NotifyIcon) SetIcon(hIcon win.HICON) error {
	data := ni.newData()
	data.UFlags |= co.NIF_ICON
	data.HIcon = hIcon
	if err := win.Shell_NotifyIcon(co.NIM_MODIFY, data); err != nil {
		return err
	}
	return nil
}

func (ni *NotifyIcon) ShowNotification(title, text string) error {
	data := ni.newData()
	data.UFlags |= co.NIF_INFO
	data.SetSzInfoTitle(title)
	data.SetSzInfo(text)
	if err := win.Shell_NotifyIcon(co.NIM_MODIFY, data); err != nil {
		return err
	}
	return nil
}

func (ni *NotifyIcon) ShowNotificationWithIcon(title, text string, hIcon uintptr) error {
	data := ni.newData()
	data.UFlags |= co.NIF_INFO
	data.SetSzInfoTitle(title)
	data.SetSzInfo(text)
	data.DwInfoFlags = co.NIIF_USER | co.NIIF_LARGE_ICON
	if err := win.Shell_NotifyIcon(co.NIM_MODIFY, data); err != nil {
		return err
	}
	return nil
}

func (ni *NotifyIcon) newData() *win.NOTIFYICONDATA {
	var nid win.NOTIFYICONDATA
	nid.SetCbSize()
	nid.UFlags = co.NIF_GUID
	nid.HWnd = ni.hwnd
	nid.GuidItem = ni.guid
	return &nid
}

func newGUID() win.GUID {
	buf := [16]byte{}
	rand.Read(buf[:])
	return *(*win.GUID)(unsafe.Pointer(&buf[0]))
}
