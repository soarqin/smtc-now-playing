package webview

import (
	"log"

	"github.com/rodrigocfd/windigo/co"
	"github.com/rodrigocfd/windigo/ui"
	"github.com/rodrigocfd/windigo/win"
	"github.com/soarqin/go-webview2"
)

type Preview struct {
	webViewWin webview2.WebView
}

type Options struct {
	URL          string
	AlwaysOnTop  bool
	OnDestroy    func()
	OnRootLoaded func(left, top, width, height int)
}

func New(opts Options) *Preview {
	width, height := ui.Dpi(600, 400)
	var webViewOptions = webview2.WebViewOptions{
		Debug:     true,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:           "SMTC Now Playing",
			Width:           uint(width),
			Height:          uint(height),
			IconId:          0,
			Center:          true,
			Borderless:      true,
			BackgroundColor: &webview2.Color{R: 0, G: 0, B: 0, A: 0},
		},
		OnDestroy: opts.OnDestroy,
	}

	wv := &Preview{}
	wv.webViewWin = webview2.NewWithOptions(webViewOptions)
	if wv.webViewWin == nil {
		log.Fatalln("Failed to load webview.")
	}

	if opts.OnRootLoaded != nil {
		wv.webViewWin.Bind("rootLoaded", opts.OnRootLoaded)
	} else {
		wv.webViewWin.Bind("rootLoaded", func(left int, top int, width int, height int) {
			wv.webViewWin.SetSize(left+width, top+height, webview2.HintFixed)
		})
	}
	wv.webViewWin.SetSize(width, height, webview2.HintFixed)
	wv.webViewWin.Navigate(opts.URL)
	wv.SetAlwaysOnTop(opts.AlwaysOnTop)

	return wv
}

func (wv *Preview) Destroy() {
	if wv.webViewWin != nil {
		wv.webViewWin.Destroy()
		wv.webViewWin = nil
	}
}

func (wv *Preview) SetAlwaysOnTop(alwaysOnTop bool) {
	if wv.webViewWin == nil {
		return
	}
	hwnd := win.HWND(wv.webViewWin.Window())
	if alwaysOnTop {
		HWND_TOPMOST := -1
		hwnd.SetWindowPos(win.HWND(HWND_TOPMOST), 0, 0, 0, 0, co.SWP_NOMOVE|co.SWP_NOSIZE|co.SWP_NOACTIVATE|co.SWP_NOOWNERZORDER)
	} else {
		HWND_NOTOPMOST := -2
		hwnd.SetWindowPos(win.HWND(HWND_NOTOPMOST), 0, 0, 0, 0, co.SWP_NOMOVE|co.SWP_NOSIZE|co.SWP_NOACTIVATE|co.SWP_NOOWNERZORDER)
	}
}

func (wv *Preview) ProcessDispatchQueue() {
	if wv.webViewWin != nil {
		wv.webViewWin.ProcessDispatchQueue()
	}
}

func (wv *Preview) Window() uintptr {
	if wv.webViewWin == nil {
		return 0
	}
	return uintptr(wv.webViewWin.Window())
}

func (wv *Preview) SetSize(x, y int, hint webview2.Hint) {
	if wv.webViewWin != nil {
		wv.webViewWin.SetSize(x, y, hint)
	}
}
