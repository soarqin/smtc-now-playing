package webview

import (
	"log/slog"
	"unsafe"

	"github.com/rodrigocfd/windigo/co"
	"github.com/rodrigocfd/windigo/ui"
	"github.com/rodrigocfd/windigo/win"
	"github.com/soarqin/go-webview2"
	"golang.org/x/sys/windows"
)

var (
	modUser32             = windows.NewLazySystemDLL("user32.dll")
	procReleaseCapture    = modUser32.NewProc("ReleaseCapture")
	procPostMessageW      = modUser32.NewProc("PostMessageW")
	procGetCursorPos      = modUser32.NewProc("GetCursorPos")
	procGetWindowLongPtrW = modUser32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW = modUser32.NewProc("SetWindowLongPtrW")
)

const (
	htLeft        uintptr = 10
	htBottomRight uintptr = 17

	wmNCLButtonDown uintptr = 0x00A1

	wsThickFrame  uintptr = 0x00040000
	wsMaximizeBox uintptr = 0x00010000
)

type screenPoint struct {
	X, Y int32
}

type Preview struct {
	webViewWin webview2.WebView
}

type Options struct {
	URL          string
	AlwaysOnTop  bool
	OnDestroy    func()
	OnRootLoaded func(left, top, width, height int)
}

// resizeEdgeInitScript injects overlay elements at window edges.
// Key details:
//   - app-region:no-drag carves out edges from any drag region
//   - background:rgba(0,0,0,0.01) forces Chromium to include these elements
//     in the composited drag region bitmap (fully transparent elements
//     are optimized out of the render tree and never affect drag regions)
//   - mousedown handlers call startResize() as a JS→Go fallback path
const resizeEdgeInitScript = `(function(){
var B=6;
var CSS=
'.__wv_re{position:fixed;-webkit-app-region:no-drag;app-region:no-drag;z-index:2147483647;background:rgba(0,0,0,0.01)}'+
'.__wv_re_n{top:0;left:'+B+'px;right:'+B+'px;height:'+B+'px;cursor:n-resize}'+
'.__wv_re_s{bottom:0;left:'+B+'px;right:'+B+'px;height:'+B+'px;cursor:s-resize}'+
'.__wv_re_w{top:'+B+'px;bottom:'+B+'px;left:0;width:'+B+'px;cursor:w-resize}'+
'.__wv_re_e{top:'+B+'px;bottom:'+B+'px;right:0;width:'+B+'px;cursor:e-resize}'+
'.__wv_re_nw{top:0;left:0;width:'+B+'px;height:'+B+'px;cursor:nw-resize}'+
'.__wv_re_ne{top:0;right:0;width:'+B+'px;height:'+B+'px;cursor:ne-resize}'+
'.__wv_re_sw{bottom:0;left:0;width:'+B+'px;height:'+B+'px;cursor:sw-resize}'+
'.__wv_re_se{bottom:0;right:0;width:'+B+'px;height:'+B+'px;cursor:se-resize}';
var M={nw:13,n:12,ne:14,w:10,e:11,sw:16,s:15,se:17};
document.addEventListener('DOMContentLoaded',function(){
  var s=document.createElement('style');
  s.textContent=CSS;
  document.head.appendChild(s);
  for(var k in M){
    var el=document.createElement('div');
    el.className='__wv_re __wv_re_'+k;
    (function(ht){
      el.addEventListener('mousedown',function(e){
        if(e.button===0){e.preventDefault();window.startResize(ht)}
      });
    })(M[k]);
    document.body.appendChild(el);
  }
  // Compute max window size from CSS constraints
  var root=document.getElementById('root');
  if(!root||!window.setMaxSize)return;
  var cs=getComputedStyle(root);
  var rect=root.getBoundingClientRect();
  // Max width: use CSS max-width if set, else current rendered width
  var maxW=root.offsetWidth;
  var cmw=cs.maxWidth;
  if(cmw&&cmw!=='none'){var pw=parseFloat(cmw);if(pw>0)maxW=pw}
  // Max height: temporarily expand to max width and measure
  var sw=root.style.width,smw=root.style.maxWidth;
  root.style.width=maxW+'px';root.style.maxWidth=maxW+'px';
  void root.offsetHeight;
  var maxH=root.offsetHeight;
  // Check CSS max-height
  var cmh=cs.maxHeight;
  if(cmh&&cmh!=='none'){var ph=parseFloat(cmh);if(ph>maxH)maxH=ph}
  root.style.width=sw;root.style.maxWidth=smw;
  var dpr=window.devicePixelRatio||1;
  window.setMaxSize(Math.round((rect.left+maxW)*dpr),Math.round((rect.top+maxH)*dpr));
});
})();`

func New(opts Options) *Preview {
	width, height := ui.Dpi(600, 400)
	var webViewOptions = webview2.WebViewOptions{
		Debug:     false,
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
		slog.Error("failed to load webview, preview disabled")
		return wv
	}

	if opts.OnRootLoaded != nil {
		wv.webViewWin.Bind("rootLoaded", opts.OnRootLoaded)
	} else {
		wv.webViewWin.Bind("rootLoaded", func(left int, top int, width int, height int) {
			wv.webViewWin.SetSize(left+width, top+height, webview2.HintFixed)
		})
	}
	wv.webViewWin.Bind("startResize", func(hitTest int) {
		wv.startResize(hitTest)
	})
	wv.webViewWin.Bind("setMaxSize", func(width int, height int) {
		wv.setMaxSize(width, height)
	})
	wv.webViewWin.SetSize(width, height, webview2.HintFixed)
	wv.webViewWin.Init(resizeEdgeInitScript)
	wv.webViewWin.Navigate(opts.URL)
	wv.SetAlwaysOnTop(opts.AlwaysOnTop)

	return wv
}

// setMaxSize sets the maximum window size constraint and strips WS_THICKFRAME
// that SetSize(HintMax) adds as a side effect.
func (wv *Preview) setMaxSize(width, height int) {
	if wv.webViewWin == nil {
		return
	}
	wv.webViewWin.SetSize(width, height, webview2.HintMax)
	wv.stripThickFrame()
}

// stripThickFrame removes WS_THICKFRAME | WS_MAXIMIZEBOX from the window style.
// SetSize(HintMax/HintNone/HintMin) adds these as a side effect which causes
// visible border and shadow on a borderless window.
func (wv *Preview) stripThickFrame() {
	hwnd := uintptr(wv.webViewWin.Window())
	gwlStyle := -16
	style, _, _ := procGetWindowLongPtrW.Call(hwnd, uintptr(gwlStyle))
	style &^= wsThickFrame | wsMaximizeBox
	procSetWindowLongPtrW.Call(hwnd, uintptr(gwlStyle), style)
}

// startResize is the JS→Go callback for edge overlay mousedown.
// Uses PostMessage (not SendMessage) to avoid blocking inside WebView2's
// MessageCallback — same approach as Wails and WebView2's own drag handling.
func (wv *Preview) startResize(hitTest int) {
	if wv.webViewWin == nil {
		return
	}
	ht := uintptr(hitTest)
	if ht < htLeft || ht > htBottomRight {
		return
	}
	hwnd := uintptr(wv.webViewWin.Window())

	var pt screenPoint
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	lParam := uintptr(pt.X&0xFFFF) | uintptr(pt.Y&0xFFFF)<<16

	procReleaseCapture.Call()
	procPostMessageW.Call(hwnd, wmNCLButtonDown, ht, lParam)
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
		hwnd.SetWindowPos(win.HWND(HWND_TOPMOST), win.POINT{X: 0, Y: 0}, win.SIZE{Cx: 0, Cy: 0}, co.SWP_NOMOVE|co.SWP_NOSIZE|co.SWP_NOACTIVATE|co.SWP_NOOWNERZORDER)
	} else {
		HWND_NOTOPMOST := -2
		hwnd.SetWindowPos(win.HWND(HWND_NOTOPMOST), win.POINT{X: 0, Y: 0}, win.SIZE{Cx: 0, Cy: 0}, co.SWP_NOMOVE|co.SWP_NOSIZE|co.SWP_NOACTIVATE|co.SWP_NOOWNERZORDER)
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

// Navigate navigates the webview to the given URL.
func (wv *Preview) Navigate(url string) {
	if wv.webViewWin != nil {
		wv.webViewWin.Navigate(url)
	}
}
