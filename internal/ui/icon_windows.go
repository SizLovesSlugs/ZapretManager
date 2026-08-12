//go:build windows

package ui

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmSetIcon  = 0x0080
	iconSmall  = 0
	iconBig    = 1
	gclpHIcon  = -14
	gclpHIconS = -34
)

func setWindowIcon(hwnd unsafe.Pointer) {
	if hwnd == nil {
		return
	}
	exe, err := os.Executable()
	if err != nil || exe == "" {
		return
	}
	exe16, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return
	}

	shell32 := windows.NewLazySystemDLL("shell32.dll")
	user32 := windows.NewLazySystemDLL("user32.dll")
	extract := shell32.NewProc("ExtractIconExW")
	sendMessage := user32.NewProc("SendMessageW")
	setClassLong := user32.NewProc("SetClassLongPtrW")

	var large, small uintptr
	r, _, _ := extract.Call(
		uintptr(unsafe.Pointer(exe16)),
		0,
		uintptr(unsafe.Pointer(&large)),
		uintptr(unsafe.Pointer(&small)),
		1,
	)
	if r == 0 {
		return
	}
	h := uintptr(hwnd)
	toIdx := func(v int32) uintptr { return uintptr(uint32(v)) }
	if large != 0 {
		sendMessage.Call(h, wmSetIcon, iconBig, large)
		setClassLong.Call(h, toIdx(gclpHIcon), large)
	}
	if small != 0 {
		sendMessage.Call(h, wmSetIcon, iconSmall, small)
		setClassLong.Call(h, toIdx(gclpHIconS), small)
	} else if large != 0 {
		sendMessage.Call(h, wmSetIcon, iconSmall, large)
		setClassLong.Call(h, toIdx(gclpHIconS), large)
	}
}
