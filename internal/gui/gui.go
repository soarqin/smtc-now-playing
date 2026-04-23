package gui

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"smtc-now-playing/internal/config"
	"smtc-now-playing/internal/server"
	"smtc-now-playing/internal/smtc"
	"smtc-now-playing/internal/updater"
	"smtc-now-playing/internal/webview"

	"github.com/rodrigocfd/windigo/co"
	"github.com/rodrigocfd/windigo/ui"
	"github.com/rodrigocfd/windigo/win"
	"github.com/soarqin/go-webview2"
)

var log = slog.With("subsystem", "gui")

type Gui struct {
	wnd                  *ui.Main
	lblPort              *ui.Static
	portEdit             *ui.Edit
	portUd               *ui.UpDown
	lblTheme             *ui.Static
	themeCombo           *ui.ComboBox
	deviceCombo          *ui.ComboBox
	btnStart             *ui.Button
	cbAutoStart          *ui.CheckBox
	cbStartMinimized     *ui.CheckBox
	cbShowPreviewWindow  *ui.CheckBox
	cbPreviewAlwaysOnTop *ui.CheckBox
	infoText             *ui.Edit

	cfg           *config.Config
	cfgPath       string
	srv           *server.Server
	smtcSvc       server.SMTCService
	serverStarted bool
	ctx           context.Context

	webViewWin        *webview.Preview
	msgTaskbarCreated co.WM
}

var notifyIcon *NotifyIcon

const (
	SYSTRAY_MENU_SHOW_HIDE  = 1001
	SYSTRAY_MENU_START_STOP = 1002
	SYSTRAY_MENU_EXIT       = 1003

	// wmSessionsChanged and wmDeviceChanged use offsets +2 and +3 because
	// NotifyIconMsg (systray.go) already occupies WM_APP+1.
	wmSessionsChanged        = co.WM_APP + 2
	wmDeviceChanged          = co.WM_APP + 3
	SYSTRAY_MENU_DEVICE_BASE = 2001 // device menu items start at 2001
)

func New(cfg *config.Config, srv *server.Server, smtcSvc server.SMTCService, version string) *Gui {
	os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", "--enable-features=msWebView2EnableDraggableRegions")

	cfgPath, err := config.ResolvePath()
	if err != nil {
		slog.Warn("failed to resolve config path for saving", "err", err)
		cfgPath = ""
	}

	optsMain := ui.OptsMain()
	if cfg.UI.StartMinimized {
		optsMain.CmdShow(co.SW_HIDE)
	}
	wnd := ui.NewMain(
		optsMain.
			Title("SMTC Now Playing v" + version).
			Size(ui.Dpi(370, 285)).
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
	ui.NewStatic(
		wnd,
		ui.OptsStatic().
			Text("Device").
			Position(ui.Dpi(10, 47)),
	)
	deviceCombo := ui.NewComboBox(
		wnd,
		ui.OptsComboBox().
			Position(ui.Dpi(60, 45)).
			Width(ui.DpiX(300)),
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
			Position(ui.Dpi(10, 75)),
	)
	cbStartMinimized := ui.NewCheckBox(
		wnd,
		ui.OptsCheckBox().
			Text("Start minimized").
			Position(ui.Dpi(10, 95)),
	)
	cbShowPreviewWindow := ui.NewCheckBox(
		wnd,
		ui.OptsCheckBox().
			Text("Show preview").
			Position(ui.Dpi(10, 115)),
	)
	cbPreviewAlwaysOnTop := ui.NewCheckBox(
		wnd,
		ui.OptsCheckBox().
			Text("Preview always on top").
			Position(ui.Dpi(10, 135)),
	)
	urlText := ui.NewEdit(
		wnd,
		ui.OptsEdit().
			CtrlStyle(co.ES_AUTOHSCROLL|co.ES_READONLY|co.ES_MULTILINE|co.ES_AUTOVSCROLL|co.ES(co.WS_VSCROLL)).
			Text("").
			Position(ui.Dpi(10, 165)).
			Width(ui.DpiX(350)).
			Height(ui.DpiY(100)),
	)

	me := &Gui{
		wnd: wnd, lblPort: lblPort, portEdit: portEdit, portUd: ud,
		lblTheme: lblTheme, themeCombo: themeCombo, deviceCombo: deviceCombo, btnStart: btnStart,
		cbAutoStart: cbAutoStart, cbStartMinimized: cbStartMinimized,
		cbShowPreviewWindow: cbShowPreviewWindow, cbPreviewAlwaysOnTop: cbPreviewAlwaysOnTop,
		infoText: urlText,
		cfg:      cfg,
		cfgPath:  cfgPath,
		srv:      srv,
		smtcSvc:  smtcSvc,
	}

	// Register TaskbarCreated message before event handlers so the value
	// is available when Wm() captures it for dispatch.
	// If registration fails (extremely unlikely), log and continue — the
	// only feature we lose is auto-restoring the tray icon after an
	// Explorer crash, which is never worth bringing the whole app down.
	var regErr error
	me.msgTaskbarCreated, regErr = win.RegisterWindowMessage("TaskbarCreated")
	if regErr != nil {
		slog.Warn("RegisterWindowMessage(TaskbarCreated) failed; tray icon will not auto-restore after Explorer restart", "err", regErr)
		me.msgTaskbarCreated = 0
	}

	me.events()

	// Check for updates in the background without blocking startup.
	go func() {
		info, err := updater.CheckForUpdate(context.Background(), version, updater.DefaultAPIURL)
		if err != nil {
			slog.Warn("update check failed", "err", err)
			return
		}
		if info != nil && info.Available {
			slog.Info("update available", "version", info.Version, "url", info.URL)
		}
	}()

	return me
}

func (me *Gui) events() {
	me.wnd.On().WmCreate(func(p ui.WmCreate) int {
		me.portUd.SetValue(me.cfg.Server.Port)
		me.portEdit.SetText(fmt.Sprintf("%d", me.cfg.Server.Port))
		me.cbAutoStart.SetCheck(me.cfg.UI.AutoStart)
		me.cbStartMinimized.SetCheck(me.cfg.UI.StartMinimized)
		me.cbShowPreviewWindow.SetCheck(me.cfg.UI.ShowPreviewWindow)
		me.cbPreviewAlwaysOnTop.SetCheck(me.cfg.UI.PreviewAlwaysOnTop)
		me.btnStart.Hwnd().EnableWindow(true)

		themes, err := os.ReadDir("themes")
		if err != nil {
			me.wnd.Hwnd().MessageBox(err.Error(), "Error", co.MB_ICONERROR)
			return 0
		}
		toSelect := 0
		i := 0
		for _, theme := range themes {
			me.themeCombo.AddItem(theme.Name())
			if theme.Name() == me.cfg.UI.Theme {
				toSelect = i
			}
			i++
		}
		me.themeCombo.SelectIndex(toSelect)

		notifyIcon, err = NewNotifyIcon(me.wnd.Hwnd())
		if err != nil {
			me.wnd.Hwnd().MessageBox(err.Error(), "Fatal Error", co.MB_ICONERROR)
			os.Exit(1)
		}

		notifyIcon.SetTooltip("Smtc Now Playing")
		hInstance, err := win.GetModuleHandle("")
		if err != nil {
			me.wnd.Hwnd().MessageBox(err.Error(), "Fatal Error", co.MB_ICONERROR)
			os.Exit(1)
		}
		hIcon, err := hInstance.LoadIcon(win.IconResStr("APP"))
		if err == nil {
			notifyIcon.SetIcon(hIcon)
			me.wnd.Hwnd().SendMessage(co.WM_SETICON, 0, win.LPARAM(hIcon))
			me.wnd.Hwnd().SendMessage(co.WM_SETICON, 1, win.LPARAM(hIcon))
		}

		// Start bridge goroutine: translate SMTC events into WM messages
		// so the main thread's message loop handles them on the UI thread.
		hwnd := me.wnd.Hwnd()
		sub := me.smtcSvc.Subscribe(16)
		go func() {
			defer me.smtcSvc.Unsubscribe(sub)
			for ev := range sub {
				switch ev.(type) {
				case smtc.SessionsChangedEvent:
					hwnd.PostMessage(wmSessionsChanged, 0, 0)
				case smtc.DeviceChangedEvent:
					hwnd.PostMessage(wmDeviceChanged, 0, 0)
				}
			}
		}()

		// Watch context for external cancellation and post WM_CLOSE so the
		// message loop exits cleanly on the UI thread.
		if me.ctx != nil {
			go func() {
				<-me.ctx.Done()
				hwnd.PostMessage(co.WM_CLOSE, 0, 0)
			}()
		}

		if me.cfg.UI.AutoStart {
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
			isServerRunning := me.serverStarted

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

			nextPos := 2
			if me.serverStarted {
				sessions := me.srv.GetSessions()
				if len(sessions) > 0 {
					deviceMenu, dErr := win.CreatePopupMenu()
					if dErr == nil {
						// Track whether we successfully attached deviceMenu
						// to popup. If we didn't, we must DestroyMenu it
						// ourselves — popup.DestroyMenu only cascades to
						// submenus that are actually attached.
						attached := false
						currentDevice := me.cfg.SMTC.SelectedDevice
						for i, sess := range sessions {
							itemState := co.MFS_UNCHECKED
							if sess.AppID == currentDevice || (currentDevice == "" && i == 0) {
								itemState = co.MFS_CHECKED
							}
							itemPtr, iErr := syscall.UTF16PtrFromString(sess.Name)
							if iErr != nil {
								continue
							}
							dmi := &win.MENUITEMINFO{
								FMask:      co.MIIM_STRING | co.MIIM_ID | co.MIIM_STATE | co.MIIM_FTYPE,
								FType:      co.MFT_STRING | co.MFT_RADIOCHECK,
								FState:     itemState,
								WId:        uint32(SYSTRAY_MENU_DEVICE_BASE + i),
								DwTypeData: itemPtr,
							}
							dmi.SetCbSize()
							deviceMenu.InsertMenuItemByPos(i, dmi)
						}
						subLabelPtr, sErr := syscall.UTF16PtrFromString("&Device")
						if sErr == nil {
							dmi := &win.MENUITEMINFO{
								FMask:      co.MIIM_STRING | co.MIIM_SUBMENU,
								FType:      co.MFT_STRING,
								HSubMenu:   deviceMenu,
								DwTypeData: subLabelPtr,
							}
							dmi.SetCbSize()
							popup.InsertMenuItemByPos(nextPos, dmi)
							nextPos++
							attached = true
						}
						if !attached {
							// UTF16 conversion failed — detach the submenu
							// we built so it's not leaked as an orphan HMENU.
							deviceMenu.DestroyMenu()
						}
					}
				}
			}

			mii = &win.MENUITEMINFO{
				FMask: co.MIIM_FTYPE,
				FType: co.MFT_SEPARATOR,
			}
			mii.SetCbSize()
			popup.InsertMenuItemByPos(nextPos, mii)
			nextPos++

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
			popup.InsertMenuItemByPos(nextPos, mii)

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
					if err := me.cfg.Save(me.cfgPath); err != nil {
						slog.Warn("failed to save config", "err", err)
					}
					me.startWebServer()
				}
			case SYSTRAY_MENU_EXIT:
				me.wnd.Hwnd().DestroyWindow()
			default:
				if res >= SYSTRAY_MENU_DEVICE_BASE {
					idx := int(res) - SYSTRAY_MENU_DEVICE_BASE
					sessions := me.srv.GetSessions()
					if idx >= 0 && idx < len(sessions) {
						appID := sessions[idx].AppID
						me.srv.SelectDevice(appID)
						me.cfg.SMTC.SelectedDevice = appID
						if err := me.cfg.Save(me.cfgPath); err != nil {
							slog.Warn("failed to save config", "err", err)
						}
					}
				}
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
		me.destroyWebView()
	})

	me.themeCombo.On().CbnSelChange(func() {
		if me.serverStarted {
			me.srv.SetTheme(me.themeCombo.CurrentText())
			me.cfg.UI.Theme = me.themeCombo.CurrentText()
			if err := me.cfg.Save(me.cfgPath); err != nil {
				slog.Warn("failed to save config", "err", err)
			}
			// Refresh preview with cache-busting URL to load the new theme
			if me.webViewWin != nil {
				me.webViewWin.Navigate(fmt.Sprintf("http://127.0.0.1:%s?_t=%d", me.portEdit.Text(), time.Now().UnixMilli()))
			}
		}
	})

	me.deviceCombo.On().CbnSelChange(func() {
		idx := me.deviceCombo.SelectedIndex()
		sessions := me.srv.GetSessions()
		if idx >= 0 && idx < len(sessions) {
			appID := sessions[idx].AppID
			me.srv.SelectDevice(appID)
			me.cfg.SMTC.SelectedDevice = appID
			if err := me.cfg.Save(me.cfgPath); err != nil {
				slog.Warn("failed to save config", "err", err)
			}
		}
	})

	me.btnStart.On().BnClicked(func() {
		if me.serverStarted {
			me.stopWebServer()
			return
		}
		me.syncConfig()
		if err := me.cfg.Save(me.cfgPath); err != nil {
			slog.Warn("failed to save config", "err", err)
		}
		me.startWebServer()
	})

	me.cbAutoStart.On().BnClicked(func() {
		me.syncConfig()
		if err := me.cfg.Save(me.cfgPath); err != nil {
			slog.Warn("failed to save config", "err", err)
		}
	})

	me.cbStartMinimized.On().BnClicked(func() {
		me.syncConfig()
		if err := me.cfg.Save(me.cfgPath); err != nil {
			slog.Warn("failed to save config", "err", err)
		}
	})

	me.cbShowPreviewWindow.On().BnClicked(func() {
		me.syncConfig()
		if err := me.cfg.Save(me.cfgPath); err != nil {
			slog.Warn("failed to save config", "err", err)
		}
		if !me.serverStarted {
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
		if err := me.cfg.Save(me.cfgPath); err != nil {
			slog.Warn("failed to save config", "err", err)
		}
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

	me.wnd.On().Wm(wmSessionsChanged, func(p ui.Wm) uintptr {
		sessions := me.srv.GetSessions()
		me.deviceCombo.DeleteAllItems()
		selectedIdx := 0
		for i, sess := range sessions {
			me.deviceCombo.AddItem(sess.Name)
			if sess.AppID == me.cfg.SMTC.SelectedDevice {
				selectedIdx = i
			}
		}
		if len(sessions) > 0 {
			me.deviceCombo.SelectIndex(selectedIdx)
		}
		return 0
	})

	me.wnd.On().Wm(wmDeviceChanged, func(p ui.Wm) uintptr {
		sessions := me.srv.GetSessions()
		selectedDevice := me.cfg.SMTC.SelectedDevice
		for i, sess := range sessions {
			if sess.AppID == selectedDevice {
				me.deviceCombo.SelectIndex(i)
				break
			}
		}
		return 0
	})
}

func (me *Gui) syncConfig() {
	port, err := strconv.Atoi(me.portEdit.Text())
	if err == nil {
		me.cfg.Server.Port = port
	}
	me.cfg.UI.Theme = me.themeCombo.CurrentText()
	me.cfg.UI.AutoStart = me.cbAutoStart.IsChecked()
	me.cfg.UI.StartMinimized = me.cbStartMinimized.IsChecked()
	me.cfg.UI.ShowPreviewWindow = me.cbShowPreviewWindow.IsChecked()
	me.cfg.UI.PreviewAlwaysOnTop = me.cbPreviewAlwaysOnTop.IsChecked()
}

func (me *Gui) startWebServer() {
	me.serverStarted = true

	// Compute listening addresses from the server's bound address.
	addr := me.srv.Address()
	var port string
	if strings.HasPrefix(addr, "0.0.0.0:") {
		port = addr[len("0.0.0.0:"):]
	} else if strings.HasPrefix(addr, ":") {
		port = addr[1:]
	}

	addresses := []string{}
	if port != "" {
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			me.wnd.Hwnd().MessageBox(err.Error(), "Error", co.MB_ICONERROR)
			return
		}
		for _, iaddr := range addrs {
			saddr := iaddr.String()
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
	me.serverStarted = false
	me.destroyWebView()
	me.infoText.SetText("")
	me.btnStart.SetText("&Start")
	me.btnStart.Hwnd().EnableWindow(true)
}

func (me *Gui) createWebView() {
	if me.webViewWin != nil {
		return
	}
	ctx := me.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	wv, err := webview.New(ctx, webview.Options{
		URL:         fmt.Sprintf("http://127.0.0.1:%s?_t=%d", me.portEdit.Text(), time.Now().UnixMilli()),
		AlwaysOnTop: me.cbPreviewAlwaysOnTop.IsChecked(),
		OnDestroy: func() {
			if !me.serverStarted {
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
	if err != nil {
		slog.Warn("failed to create WebView2 preview", "err", err)
		return
	}
	me.webViewWin = wv
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
		hwnd.SetWindowPos(win.HWND(HWND_TOPMOST), win.POINT{X: 0, Y: 0}, win.SIZE{Cx: 0, Cy: 0}, co.SWP_NOMOVE|co.SWP_NOSIZE|co.SWP_NOACTIVATE|co.SWP_NOOWNERZORDER)
	} else {
		HWND_NOTOPMOST := -2
		hwnd.SetWindowPos(win.HWND(HWND_NOTOPMOST), win.POINT{X: 0, Y: 0}, win.SIZE{Cx: 0, Cy: 0}, co.SWP_NOMOVE|co.SWP_NOSIZE|co.SWP_NOACTIVATE|co.SWP_NOOWNERZORDER)
	}
}

// Run locks the calling goroutine to its OS thread (required for the Win32
// message loop), then runs the windigo main window until the window is
// destroyed or ctx is cancelled. Returns context.Canceled on normal close so
// that errgroup propagates the shutdown signal to other services.
func (me *Gui) Run(ctx context.Context) error {
	runtime.LockOSThread()
	me.ctx = ctx
	code := me.wnd.RunAsMain()
	if code != 0 {
		return fmt.Errorf("gui exited with code %d", code)
	}
	// Return context.Canceled so the errgroup cancels the shared context,
	// signalling smtcSvc and server to stop when the window closes.
	return context.Canceled
}
