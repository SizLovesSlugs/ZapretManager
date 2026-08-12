//go:build windows

package ui

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	gclpHbrBackground = int32(-10)
	colorBg           = 0x000E0B0A // #0a0b0e as COLORREF (0x00BBGGRR)
	// WEBVIEW2_DEFAULT_BACKGROUND_COLOR: AARRGGBB for #0a0b0e
	webviewDefaultBgARGB = "FF0A0B0E"
)

func paintWindowDark(hwnd unsafe.Pointer) {
	if hwnd == nil {
		return
	}
	h := uintptr(hwnd)
	user32 := windows.NewLazySystemDLL("user32.dll")
	gdi32 := windows.NewLazySystemDLL("gdi32.dll")

	brush, _, _ := gdi32.NewProc("CreateSolidBrush").Call(uintptr(colorBg))
	if brush == 0 {
		return
	}
	toIdx := func(v int32) uintptr { return uintptr(uint32(v)) }
	user32.NewProc("SetClassLongPtrW").Call(h, toIdx(gclpHbrBackground), brush)

	var rc struct{ Left, Top, Right, Bottom int32 }
	user32.NewProc("GetClientRect").Call(h, uintptr(unsafe.Pointer(&rc)))
	hdc, _, _ := user32.NewProc("GetDC").Call(h)
	if hdc != 0 {
		user32.NewProc("FillRect").Call(hdc, uintptr(unsafe.Pointer(&rc)), brush)
		user32.NewProc("ReleaseDC").Call(h, hdc)
	}
	user32.NewProc("InvalidateRect").Call(h, 0, 1)
	user32.NewProc("UpdateWindow").Call(h)
}
