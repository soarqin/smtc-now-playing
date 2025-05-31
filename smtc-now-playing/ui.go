package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rodrigocfd/windigo/ui"
	"github.com/rodrigocfd/windigo/win"
	"github.com/rodrigocfd/windigo/win/co"
)

// This struct represents our main window.
type Gui struct {
	wnd       *ui.Main
	lblPort   *ui.Static
	portEdit  *ui.Edit
	portUd    *ui.UpDown
	btnStart  *ui.Button
	urlText   *ui.Edit
	exigGroup ProcessExitGroup
	srv       *WebServer
	monitor   *Monitor
}

var notifyIcon *NotifyIcon
var msgTaskbarCreated co.WM

// Creates a new instance of our main window.
func NewGui(g ProcessExitGroup) *Gui {
	wnd := ui.NewMain( // create the main window
		ui.OptsMain().
			Title("SMTC Now Playing").
			Size(ui.Dpi(340, 100)).
			ClassIconId(101), // ID of icon resource, see resources folder
	)

	lblPort := ui.NewStatic( // create the child controls
		wnd,
		ui.OptsStatic().
			Text("Port").
			Position(ui.Dpi(10, 22)),
	)
	portEdit := ui.NewEdit(
		wnd,
		ui.OptsEdit().
			Text("11451").
			CtrlStyle(co.ES_NUMBER|co.ES_RIGHT).
			Position(ui.Dpi(50, 20)).
			Width(ui.DpiX(80)),
	)
	ud := ui.NewUpDown(
		wnd,
		ui.OptsUpDown().
			CtrlStyle(co.UDS_AUTOBUDDY|co.UDS_ALIGNRIGHT|co.UDS_SETBUDDYINT|co.UDS_NOTHOUSANDS).
			Position(ui.Dpi(230, 20)).Range(1024, 32767).Value(11451),
	)
	btnStart := ui.NewButton(
		wnd,
		ui.OptsButton().
			Text("&Start").
			Position(ui.Dpi(240, 19)),
	)
	urlText := ui.NewEdit(
		wnd,
		ui.OptsEdit().
			CtrlStyle(co.ES_AUTOHSCROLL|co.ES_READONLY).
			Text("").
			Position(ui.Dpi(10, 50)).
			Width(ui.DpiX(320)),
	)

	me := &Gui{wnd, lblPort, portEdit, ud, btnStart, urlText, g, nil, nil}
	me.events()
	return me
}

func (me *Gui) events() {
	me.wnd.On().WmCreate(func(p ui.WmCreate) int {
		var err error
		msgTaskbarCreated, err = win.RegisterWindowMessage("TaskbarCreated")
		if err != nil {
			panic(err)
		}

		notifyIcon, err = NewNotifyIcon(me.wnd.Hwnd())
		if err != nil {
			panic(err)
		}

		notifyIcon.SetTooltip("Smtc Now Playing")
		hInstance, err := win.GetModuleHandle("")
		if err != nil {
			panic(err)
		}
		hIcon, err := hInstance.LoadIcon(win.IconResStr("APP"))
		if err == nil {
			notifyIcon.SetIcon(hIcon)
			me.wnd.Hwnd().SendMessage(co.WM_SETICON, 0, win.LPARAM(hIcon))
			me.wnd.Hwnd().SendMessage(co.WM_SETICON, 1, win.LPARAM(hIcon))
		}
		go me.monitorProcess()
		return 0
	})

	me.wnd.On().Wm(msgTaskbarCreated, func(p ui.Wm) uintptr {
		if notifyIcon != nil {
			notifyIcon.AddAfterExplorerCrash()
		}
		return 0
	})

	me.wnd.On().WmDestroy(func() {
		if notifyIcon != nil {
			notifyIcon.Dispose()
			notifyIcon = nil
		}
	})

	me.btnStart.On().BnClicked(func() {
	})
}

func (me *Gui) monitorProcess() {
	// Get current directory
	dir, err := os.Getwd()
	// Add .mod to PATHEXT
	os.Setenv("PATHEXT", os.Getenv("PATHEXT")+";.mod")
	if err != nil {
		me.wnd.Hwnd().MessageBox(err.Error(), "Error", co.MB_ICONERROR)
		return
	}
	me.monitor = NewMonitor(me.exigGroup, filepath.Join(dir, "SmtcMonitor"))
	err = me.monitor.StartProcess()
	if err != nil {
		me.wnd.Hwnd().MessageBox(err.Error(), "Error", co.MB_ICONERROR)
		return
	}
	monitorErrChan := me.monitor.GetErrorChannel()
	me.srv = NewWebServer("localhost", "11451", me.monitor)
	srvErrChan := me.srv.Error()
	go func() {
		for {
			select {
			case err := <-monitorErrChan:
				fmt.Printf("Error: %v\n", err)
			case err := <-srvErrChan:
				fmt.Printf("Web server error: %v\n", err)
				me.stopProcess()
				return
			}
		}
	}()
	me.srv.Start()
	fmt.Printf("Server started at http://%s\n", me.srv.Address())
}

func (me *Gui) stopProcess() {
	me.srv.Stop()
	me.monitor.Stop()
	me.monitor.Join()
	me.srv = nil
	me.monitor = nil
}
