package ui

import (
	"embed"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"unsafe"

	"github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
	"zapret-manager/internal/admin"
	"zapret-manager/internal/app"
	"zapret-manager/internal/version"
)

//go:embed web/*
var webFS embed.FS

func Run(a *app.App) error {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	go func() {
		if err := http.Serve(ln, http.FileServer(http.FS(sub))); err != nil {
			log.Println(err)
		}
	}()

	// Must be set before WebView2 Runtime init (avoids white flicker; AARRGGBB).
	_ = os.Setenv("WEBVIEW2_DEFAULT_BACKGROUND_COLOR", webviewDefaultBgARGB)

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  version.Title(),
			Width:  1100,
			Height: 760,
			Center: true,
		},
	})
	if w == nil {
		admin.ErrorBox(version.Title(), "Не удалось открыть окно. Установите Microsoft Edge WebView2 Runtime.")
		return errWebView
	}
	defer w.Destroy()
	paintWindowDark(w.Window())
	enableDarkTitleBar(w.Window())
	setWindowIcon(w.Window())
	w.SetSize(1100, 760, webview2.HintMin)

	// Paint WebView document dark as early as possible (before CSS/JS load).
	w.Init(`(function(){try{var s=document.createElement('style');s.textContent='html,body{background:#0a0b0e!important;color:#f4f6f8}';(document.head||document.documentElement).appendChild(s);document.documentElement.style.background='#0a0b0e';if(document.body)document.body.style.background='#0a0b0e';}catch(e){}})();`)

	_ = w.Bind("getState", a.GetState)
	_ = w.Bind("boot", a.Boot)
	_ = w.Bind("loadReleases", a.LoadReleases)
	_ = w.Bind("selectVersion", a.SelectVersion)
	_ = w.Bind("selectStrategy", a.SelectStrategy)
	_ = w.Bind("selectGameStrategy", a.SelectGameStrategy)
	_ = w.Bind("setGameBoost", a.SetGameBoost)
	_ = w.Bind("setGameFilter", a.SetGameFilter)
	_ = w.Bind("setDNSProfile", a.SetDNSProfile)
	_ = w.Bind("setTelegramWebBoost", a.SetTelegramWebBoost)
	_ = w.Bind("setServiceBoost", a.SetServiceBoost)
	_ = w.Bind("setGeoProxy", a.SetGeoProxy)
	_ = w.Bind("pingGeoProxies", a.PingGeoProxies)
	_ = w.Bind("startZapret", a.Start)
	_ = w.Bind("stopZapret", a.Stop)
	_ = w.Bind("removeZapret", a.Remove)
	_ = w.Bind("getLogs", a.GetLogs)
	_ = w.Bind("logError", a.LogError)
	_ = w.Bind("openURL", admin.OpenURL)
	_ = w.Bind("applyAppUpdate", a.ApplyAppUpdate)

	w.Navigate("http://" + ln.Addr().String() + "/")
	// Start background boot after UI navigation so the window can paint first.
	_, _ = a.Boot()
	w.Run()
	return nil
}

var errWebView = errString("webview2 unavailable")

type errString string

func (e errString) Error() string { return string(e) }

func enableDarkTitleBar(hwnd unsafe.Pointer) {
	if hwnd == nil {
		return
	}
	dwmapi := windows.NewLazySystemDLL("dwmapi.dll")
	proc := dwmapi.NewProc("DwmSetWindowAttribute")
	dark := int32(1)
	h := uintptr(hwnd)
	proc.Call(h, 20, uintptr(unsafe.Pointer(&dark)), 4)
	proc.Call(h, 19, uintptr(unsafe.Pointer(&dark)), 4)
}
