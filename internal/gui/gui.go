package gui

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/rodrigocfd/windigo/co"
	"github.com/rodrigocfd/windigo/ui"
	"github.com/rodrigocfd/windigo/win"
	"github.com/soarqin/go-webview2"
	"smtc-now-playing/internal/config"
	"smtc-now-playing/internal/server"
	"smtc-now-playing/internal/webview"
)

type Gui struct {
	wnd                  *ui.Main
	lblPort              *ui.Static
	portEdit             *ui.Edit
	portUd               *ui.UpDown
	lblTheme             *ui.Static
	themeCombo           *ui.ComboBox
	btnStart             *ui.Button
	cbAutoStart          *ui.CheckBox
	cbStartMinimized     *ui.CheckBox
	cbShowPreviewWindow  *ui.CheckBox
	cbPreviewAlwaysOnTop *ui.CheckBox
	infoText             *ui.Edit

	srv               *server.WebServer
	webViewWin        *webview.Preview
	msgTaskbarCreated co.WM
}

var notifyIcon *NotifyIcon

const (
	SYSTRAY_MENU_SHOW_HIDE  = 1001
	SYSTRAY_MENU_START_STOP = 1002
	SYSTRAY_MENU_EXIT       = 1003
)

func New() *Gui {
	os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", "--enable-features=msWebView2EnableDraggableRegions")

	optsMain := ui.OptsMain()
	if config.Get().StartMinimized {
		optsMain.CmdShow(co.SW_HIDE)
	}
	wnd := ui.NewMain(
		optsMain.
			Title("SMTC Now Playing").
			Size(ui.Dpi(370, 260)).
			ClassIconId(0),
	)

	lblPort := ui.NewStatic(
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
	cbStartMinimized := ui.NewCheckBox(
		wnd,
		ui.OptsCheckBox().
			Text("Start minimized").
			Position(ui.Dpi(10, 70)),
	)
	cbShowPreviewWindow := ui.NewCheckBox(
		wnd,
		ui.OptsCheckBox().
			Text("Show preview").
			Position(ui.Dpi(10, 90)),
	)
	cbPreviewAlwaysOnTop := ui.NewCheckBox(
		wnd,
		ui.OptsCheckBox().
			Text("Preview always on top").
			Position(ui.Dpi(10, 110)),
	)
	urlText := ui.NewEdit(
		wnd,
		ui.OptsEdit().
			CtrlStyle(co.ES_AUTOHSCROLL|co.ES_READONLY|co.ES_MULTILINE|co.ES_AUTOVSCROLL|co.ES(co.WS_VSCROLL)).
			Text("").
			Position(ui.Dpi(10, 140)).
			Width(ui.DpiX(350)).
			Height(ui.DpiY(100)),
	)

	me := &Gui{
		wnd: wnd, lblPort: lblPort, portEdit: portEdit, portUd: ud,
		lblTheme: lblTheme, themeCombo: themeCombo, btnStart: btnStart,
		cbAutoStart: cbAutoStart, cbStartMinimized: cbStartMinimized,
		cbShowPreviewWindow: cbShowPreviewWindow, cbPreviewAlwaysOnTop: cbPreviewAlwaysOnTop,
		infoText: urlText,
	}
	me.events()
	return me
}

func (me *Gui) events() {
	me.wnd.On().WmCreate(func(p ui.WmCreate) int {
		me.portUd.SetValue(config.Get().Port)
		me.portEdit.SetText(fmt.Sprintf("%d", config.Get().Port))
		me.cbAutoStart.SetCheck(config.Get().AutoStart)
		me.cbStartMinimized.SetCheck(config.Get().StartMinimized)
		me.cbShowPreviewWindow.SetCheck(config.Get().ShowPreviewWindow)
		me.cbPreviewAlwaysOnTop.SetCheck(config.Get().PreviewAlwaysOnTop)
		me.btnStart.Hwnd().EnableWindow(true)

		themes, err := os.ReadDir("themes")
		if err != nil {
			me.wnd.Hwnd().MessageBox(err.Error(), "Error", co.MB_ICONERROR)
			return 0
		}
		toSelect := 0
		i := 0
		for _, theme := range themes {
			me.themeCombo.Items.Add(theme.Name())
			if theme.Name() == config.Get().Theme {
				toSelect = i
			}
			i++
		}
		me.themeCombo.Items.Select(toSelect)

		me.msgTaskbarCreated, err = win.RegisterWindowMessage("TaskbarCreated")
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
		if config.Get().AutoStart {
			me.btnStart.TriggerClick()
		}
		return 0
	})

	me.wnd.On().Wm(me.msgTaskbarCreated, func(p ui.Wm) uintptr {
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

			isWindowVisible := false
			if style, _ := me.wnd.Hwnd().GetWindowLongPtr(co.GWLP_STYLE); style&uintptr(co.WS_VISIBLE) != 0 {
				isWindowVisible = true
			}
			isServerRunning := me.srv != nil

			showHideLabel := "Hide &Window"
			if !isWindowVisible {
				showHideLabel = "Show &Window"
			}
			utf16Ptr, err := syscall.UTF16PtrFromString(showHideLabel)
			if err != nil {
				popup.DestroyMenu()
				break
			}
			mii := &win.MENUITEMINFO{
				FMask:      co.MIIM_STRING | co.MIIM_ID,
				FType:      co.MFT_STRING,
				WId:        SYSTRAY_MENU_SHOW_HIDE,
				DwTypeData: utf16Ptr,
			}
			mii.SetCbSize()
			popup.InsertMenuItemByPos(0, mii)

			startStopLabel := "Stop &Server"
			if !isServerRunning {
				startStopLabel = "Start &Server"
			}
			utf16Ptr, err = syscall.UTF16PtrFromString(startStopLabel)
			if err != nil {
				popup.DestroyMenu()
				break
			}
			mii = &win.MENUITEMINFO{
				FMask:      co.MIIM_STRING | co.MIIM_ID,
				FType:      co.MFT_STRING,
				WId:        SYSTRAY_MENU_START_STOP,
				DwTypeData: utf16Ptr,
			}
			mii.SetCbSize()
			popup.InsertMenuItemByPos(1, mii)

			mii = &win.MENUITEMINFO{
				FMask: co.MIIM_FTYPE,
				FType: co.MFT_SEPARATOR,
			}
			mii.SetCbSize()
			popup.InsertMenuItemByPos(2, mii)

			utf16Ptr, err = syscall.UTF16PtrFromString("E&xit")
			if err != nil {
				popup.DestroyMenu()
				break
			}
			mii = &win.MENUITEMINFO{
				FMask:      co.MIIM_STRING | co.MIIM_ID,
				FType:      co.MFT_STRING,
				WId:        SYSTRAY_MENU_EXIT,
				DwTypeData: utf16Ptr,
			}
			mii.SetCbSize()
			popup.InsertMenuItemByPos(3, mii)

			pos, err := win.GetCursorPos()
			if err != nil {
				popup.DestroyMenu()
				break
			}

			res, err := popup.TrackPopupMenu(co.TPM_RETURNCMD|co.TPM_NONOTIFY, int(pos.X), int(pos.Y), me.wnd.Hwnd())
			if err != nil {
				popup.DestroyMenu()
				break
			}

			switch res {
			case SYSTRAY_MENU_SHOW_HIDE:
				if isWindowVisible {
					me.wnd.Hwnd().ShowWindow(co.SW_HIDE)
				} else {
					me.wnd.Hwnd().ShowWindow(co.SW_SHOWNORMAL)
					me.wnd.Hwnd().SetForegroundWindow()
				}
			case SYSTRAY_MENU_START_STOP:
				if isServerRunning {
					me.stopWebServer()
				} else {
					me.syncConfig()
					config.Save()
					me.startWebServer()
				}
			case SYSTRAY_MENU_EXIT:
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
		me.stopWebServer()
	})

	me.themeCombo.On().CbnSelChange(func() {
		if me.srv != nil {
			me.srv.SetTheme(me.themeCombo.Text())
			config.Get().Theme = me.themeCombo.Text()
			config.Save()
		}
	})

	me.btnStart.On().BnClicked(func() {
		if me.srv != nil {
			me.stopWebServer()
			return
		}
		me.syncConfig()
		config.Save()
		me.startWebServer()
	})

	me.cbAutoStart.On().BnClicked(func() {
		me.syncConfig()
		config.Save()
	})

	me.cbStartMinimized.On().BnClicked(func() {
		me.syncConfig()
		config.Save()
	})

	me.cbShowPreviewWindow.On().BnClicked(func() {
		me.syncConfig()
		config.Save()
		if me.srv == nil {
			return
		}
		if me.cbShowPreviewWindow.IsChecked() {
			me.createWebView()
		} else {
			me.destroyWebView()
		}
	})

	me.cbPreviewAlwaysOnTop.On().BnClicked(func() {
		me.syncConfig()
		config.Save()
		if me.webViewWin == nil {
			return
		}
		me.updateWebViewAlwaysOnTop()
	})

	me.wnd.On().Wm(0x8000, func(p ui.Wm) uintptr {
		if me.webViewWin != nil {
			me.webViewWin.ProcessDispatchQueue()
		}
		return 0
	})
}

func (me *Gui) syncConfig() {
	port, err := strconv.Atoi(me.portEdit.Text())
	if err == nil {
		config.Get().Port = port
	}
	config.Get().Theme = me.themeCombo.Text()
	config.Get().AutoStart = me.cbAutoStart.IsChecked()
	config.Get().StartMinimized = me.cbStartMinimized.IsChecked()
	config.Get().ShowPreviewWindow = me.cbShowPreviewWindow.IsChecked()
	config.Get().PreviewAlwaysOnTop = me.cbPreviewAlwaysOnTop.IsChecked()
}

func (me *Gui) startWebServer() {
	me.btnStart.Hwnd().EnableWindow(false)
	me.srv = server.New("0.0.0.0", me.portEdit.Text(), me.themeCombo.Text())
	srvErrChan := me.srv.Error()
	go func() {
		for err := range srvErrChan {
			me.infoText.SetText(fmt.Sprintf("Web server error: %v", err))
			me.stopWebServer()
			return
		}
	}()
	me.srv.Start()
	addr := me.srv.Address()
	addresses := []string{}
	if strings.HasPrefix(addr, "0.0.0.0:") {
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			me.wnd.Hwnd().MessageBox(err.Error(), "Error", co.MB_ICONERROR)
			return
		}
		port := addr[len("0.0.0.0:"):]
		for _, addr := range addrs {
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

	if me.cbShowPreviewWindow.IsChecked() {
		me.createWebView()
	}
}

func (me *Gui) stopWebServer() {
	me.btnStart.Hwnd().EnableWindow(false)

	if me.srv != nil {
		me.srv.Stop()
		me.srv = nil
	}
	me.destroyWebView()

	me.infoText.SetText("")
	me.btnStart.SetText("&Start")
	me.btnStart.Hwnd().EnableWindow(true)
}

func (me *Gui) createWebView() {
	if me.webViewWin != nil {
		return
	}
	me.webViewWin = webview.New(webview.Options{
		URL:         "http://127.0.0.1:" + me.portEdit.Text(),
		AlwaysOnTop: me.cbPreviewAlwaysOnTop.IsChecked(),
		OnDestroy: func() {
			if me.srv == nil {
				return
			}
			if me.webViewWin != nil {
				me.webViewWin.Destroy()
				me.webViewWin = nil
			}
			me.cbShowPreviewWindow.SetCheck(false)
		},
		OnRootLoaded: func(left int, top int, width int, height int) {
			me.webViewWin.SetSize(left+width, top+height, webview2.HintFixed)
		},
	})
	me.updateWebViewAlwaysOnTop()
}

func (me *Gui) destroyWebView() {
	if me.webViewWin != nil {
		me.webViewWin.Destroy()
		me.webViewWin = nil
	}
}

func (me *Gui) updateWebViewAlwaysOnTop() {
	if me.webViewWin == nil {
		return
	}
	hwnd := win.HWND(me.webViewWin.Window())
	if me.cbPreviewAlwaysOnTop.IsChecked() {
		HWND_TOPMOST := -1
		hwnd.SetWindowPos(win.HWND(HWND_TOPMOST), 0, 0, 0, 0, co.SWP_NOMOVE|co.SWP_NOSIZE|co.SWP_NOACTIVATE|co.SWP_NOOWNERZORDER)
	} else {
		HWND_NOTOPMOST := -2
		hwnd.SetWindowPos(win.HWND(HWND_NOTOPMOST), 0, 0, 0, 0, co.SWP_NOMOVE|co.SWP_NOSIZE|co.SWP_NOACTIVATE|co.SWP_NOOWNERZORDER)
	}
}

func (me *Gui) Run() int {
	return me.wnd.RunAsMain()
}
