package main

import (
	"fmt"

	"github.com/rodrigocfd/windigo/ui"
	"github.com/rodrigocfd/windigo/win"
	"github.com/rodrigocfd/windigo/win/co"
)

// func main() {
// 	runtime.LockOSThread() // important: Windows GUI is single-threaded

// 	myWindow := NewMyWindow() // instantiate
// 	myWindow.wnd.RunAsMain()  // ...and run
// }

// This struct represents our main window.
type Gui struct {
	wnd     *ui.Main
	lblName *ui.Static
	txtName *ui.Edit
	btnShow *ui.Button

	windowCreated chan struct{}
}

var notifyIcon *NotifyIcon
var msgTaskbarCreated co.WM

// Creates a new instance of our main window.
func NewGui() *Gui {
	wnd := ui.NewMain( // create the main window
		ui.OptsMain().
			Title("Hello you").
			Size(ui.Dpi(340, 80)).
			ClassIconId(101), // ID of icon resource, see resources folder
	)

	lblName := ui.NewStatic( // create the child controls
		wnd,
		ui.OptsStatic().
			Text("Your name").
			Position(ui.Dpi(10, 22)),
	)
	txtName := ui.NewEdit(
		wnd,
		ui.OptsEdit().
			Position(ui.Dpi(80, 20)).
			Width(ui.DpiX(150)),
	)
	btnShow := ui.NewButton(
		wnd,
		ui.OptsButton().
			Text("&Show").
			Position(ui.Dpi(240, 19)),
	)

	me := &Gui{wnd, lblName, txtName, btnShow, make(chan struct{})}
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

		me.windowCreated <- struct{}{}
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

	me.btnShow.On().BnClicked(func() {
		msg := fmt.Sprintf("Hello, %s!", me.txtName.Text())
		me.wnd.Hwnd().MessageBox(msg, "Saying hello", co.MB_ICONINFORMATION)
	})
}

func (me *Gui) WindowCreated() <-chan struct{} {
	return me.windowCreated
}
