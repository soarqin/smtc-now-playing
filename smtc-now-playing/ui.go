package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/rodrigocfd/windigo/ui"
	"github.com/rodrigocfd/windigo/win"
	"github.com/rodrigocfd/windigo/win/co"
)

// This struct represents our main window.
type Gui struct {
	wnd         *ui.Main
	lblPort     *ui.Static
	portEdit    *ui.Edit
	portUd      *ui.UpDown
	lblTheme    *ui.Static
	themeCombo  *ui.ComboBox
	btnStart    *ui.Button
	cbAutoStart *ui.CheckBox
	infoText    *ui.Edit
	exigGroup   ProcessExitGroup
	srv         *WebServer
	monitor     *Monitor
}

var notifyIcon *NotifyIcon
var msgTaskbarCreated co.WM

// Creates a new instance of our main window.
func NewGui(g ProcessExitGroup) *Gui {
	LoadConfig()

	wnd := ui.NewMain( // create the main window
		ui.OptsMain().
			Title("SMTC Now Playing").
			Size(ui.Dpi(370, 200)).
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
			Position(ui.Dpi(40, 20)).
			Width(ui.DpiX(80)),
	)
	ud := ui.NewUpDown(
		wnd,
		ui.OptsUpDown().
			CtrlStyle(co.UDS_AUTOBUDDY|co.UDS_ALIGNRIGHT|co.UDS_SETBUDDYINT|co.UDS_NOTHOUSANDS).
			Position(ui.Dpi(120, 20)).Range(1024, 32767).Value(11451),
	)
	lblTheme := ui.NewStatic(
		wnd,
		ui.OptsStatic().
			Text("Theme").
			Position(ui.Dpi(130, 22)),
	)
	themeCombo := ui.NewComboBox(
		wnd,
		ui.OptsComboBox().
			Position(ui.Dpi(180, 20)).
			Width(ui.DpiX(80)),
	)
	btnStart := ui.NewButton(
		wnd,
		ui.OptsButton().
			Text("&Start").
			Position(ui.Dpi(270, 19)),
	)
	cbAutoStart := ui.NewCheckBox(
		wnd,
		ui.OptsCheckBox().
			Text("Auto start server").
			Position(ui.Dpi(10, 50)),
	)
	urlText := ui.NewEdit(
		wnd,
		ui.OptsEdit().
			CtrlStyle(co.ES_AUTOHSCROLL|co.ES_READONLY|co.ES_MULTILINE|co.ES_AUTOVSCROLL|co.ES(co.WS_VSCROLL)).
			Text("").
			Position(ui.Dpi(10, 80)).
			Width(ui.DpiX(350)).
			Height(ui.DpiY(100)),
	)

	me := &Gui{wnd, lblPort, portEdit, ud, lblTheme, themeCombo, btnStart, cbAutoStart, urlText, g, nil, nil}
	me.events()
	return me
}

func (me *Gui) events() {
	me.wnd.On().WmCreate(func(p ui.WmCreate) int {
		me.portUd.SetValue(config.Port)
		me.portEdit.SetText(fmt.Sprintf("%d", config.Port))
		me.cbAutoStart.SetCheck(config.AutoStart)

		// List themes in folder and add them to combobox
		themes, err := os.ReadDir("themes")
		if err != nil {
			me.wnd.Hwnd().MessageBox(err.Error(), "Error", co.MB_ICONERROR)
			return 0
		}
		toSelect := 0
		i := 0
		for _, theme := range themes {
			me.themeCombo.Items.Add(theme.Name())
			if theme.Name() == config.Theme {
				toSelect = i
			}
			i++
		}
		me.themeCombo.Items.Select(toSelect)

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
		if config.AutoStart {
			me.btnStart.TriggerClick()
		}
		return 0
	})

	me.wnd.On().Wm(msgTaskbarCreated, func(p ui.Wm) uintptr {
		if notifyIcon != nil {
			notifyIcon.AddAfterExplorerCrash()
		}
		return 0
	})

	me.wnd.On().Wm(NotifyIconMsg, func(p ui.Wm) uintptr {
		switch co.WM(p.LParam.LoWord()) {
		case co.WM_LBUTTONDBLCLK:
			if style, _ := me.wnd.Hwnd().GetWindowLongPtr(co.GWLP_STYLE); style&uintptr(co.WS_VISIBLE) != 0 {
				me.wnd.Hwnd().ShowWindow(co.SW_HIDE)
			} else {
				me.wnd.Hwnd().ShowWindow(co.SW_SHOWNORMAL)
				me.wnd.Hwnd().SetForegroundWindow()
			}
		case co.WM_RBUTTONUP:
			popup, err := win.CreatePopupMenu()
			if err != nil {
				break
			}
			utf16Ptr, err := syscall.UTF16PtrFromString("E&xit")
			if err != nil {
				break
			}
			mii := &win.MENUITEMINFO{
				FMask:      co.MIIM_STRING | co.MIIM_ID,
				FType:      co.MFT_STRING,
				WId:        1,
				DwTypeData: utf16Ptr,
			}
			mii.SetCbSize()
			popup.InsertMenuItemByPos(0, mii)
			pos, err := win.GetCursorPos()
			if err != nil {
				break
			}
			res, err := popup.TrackPopupMenu(co.TPM_RETURNCMD|co.TPM_NONOTIFY, int(pos.X), int(pos.Y), me.wnd.Hwnd())
			if err != nil {
				break
			}
			switch res {
			case 1:
				me.wnd.Hwnd().DestroyWindow()
			}
			popup.DestroyMenu()
		}
		return 0
	})

	me.wnd.On().WmSize(func(p ui.WmSize) {
		if p.Request() == co.SIZE_REQ_MINIMIZED {
			me.wnd.Hwnd().ShowWindow(co.SW_HIDE)
		}
	})

	me.wnd.On().WmDestroy(func() {
		if notifyIcon != nil {
			notifyIcon.Dispose()
			notifyIcon = nil
		}
		me.stopProcess()
	})

	me.themeCombo.On().CbnSelChange(func() {
		if me.srv != nil {
			me.srv.SetTheme(me.themeCombo.Text())
		}
	})

	me.btnStart.On().BnClicked(func() {
		if me.monitor != nil {
			me.stopProcess()
			return
		}
		me.syncConfig()
		SaveConfig()
		me.monitorProcess()
	})

	me.cbAutoStart.On().BnClicked(func() {
		me.syncConfig()
		SaveConfig()
	})
}

func (me *Gui) syncConfig() {
	port, err := strconv.Atoi(me.portEdit.Text())
	if err == nil {
		config.Port = port
	}
	config.Theme = me.themeCombo.Text()
	config.AutoStart = me.cbAutoStart.IsChecked()
}

func (me *Gui) monitorProcess() {
	me.btnStart.Hwnd().EnableWindow(false)
	// Get current directory
	dir, err := os.Getwd()
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
	me.srv = NewWebServer("0.0.0.0", me.portEdit.Text(), me.monitor, me.themeCombo.Text())
	srvErrChan := me.srv.Error()
	go func() {
		for {
			select {
			case err := <-srvErrChan:
				me.infoText.SetText(fmt.Sprintf("Web server error: %v", err))
				me.stopProcess()
				return
			case <-monitorErrChan:
			}
		}
	}()
	me.srv.Start()
	addr := me.srv.Address()
	addresses := []string{}
	if strings.HasPrefix(addr, "0.0.0.0:") {
		// Get all addresses on the machine
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			me.wnd.Hwnd().MessageBox(err.Error(), "Error", co.MB_ICONERROR)
			return
		}
		port := addr[len("0.0.0.0:"):]
		for _, addr := range addrs {
			// Check if addr is IPv4, if not, skip
			saddr := addr.String()
			ip, _, err := net.ParseCIDR(saddr)
			if err != nil {
				continue
			}
			if ip.To4() == nil {
				continue
			}
			ipstr := ip.String()
			if ipstr == "127.0.0.1" {
				addresses = append([]string{fmt.Sprintf("http://localhost:%s", port)}, addresses...)
			} else {
				addresses = append(addresses, fmt.Sprintf("http://%s:%s", ipstr, port))
			}
		}
	} else {
		addresses = append(addresses, fmt.Sprintf("http://%s", addr))
	}
	me.infoText.SetText(fmt.Sprintf("Server listening on:\r\n  %s", strings.Join(addresses, "\r\n  ")))
	me.btnStart.SetText("&Stop")
	me.btnStart.Hwnd().EnableWindow(true)
}

func (me *Gui) stopProcess() {
	me.btnStart.Hwnd().EnableWindow(false)
	if me.srv != nil {
		me.srv.Stop()
		me.monitor.Stop()
		me.monitor.Join()
		me.srv = nil
		me.monitor = nil
	}
	me.btnStart.SetText("&Start")
	me.btnStart.Hwnd().EnableWindow(true)
}
