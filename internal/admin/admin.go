package admin

import (
	"fmt"
	"os"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	elevatedOnce sync.Once
	elevated     bool
)

func IsElevated() bool {
	elevatedOnce.Do(func() {
		elevated = windows.GetCurrentProcessToken().IsElevated()
	})
	return elevated
}

func EnsureElevated() error {
	if IsElevated() {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(exe)
	cwd, _ := syscall.UTF16PtrFromString("")
	var args *uint16
	if len(os.Args) > 1 {
		joined := windows.EscapeArg(os.Args[1])
		for _, a := range os.Args[2:] {
			joined += " " + windows.EscapeArg(a)
		}
		args, _ = syscall.UTF16PtrFromString(joined)
	}
	err = windows.ShellExecute(0, verb, file, args, cwd, windows.SW_SHOWNORMAL)
	if err != nil {
		ErrorBox("Zapret Manager", "Нужны права администратора, чтобы управлять службой Zapret.")
		return fmt.Errorf("elevation cancelled")
	}
	os.Exit(0)
	return nil
}

func HideConsole() {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	user32 := windows.NewLazySystemDLL("user32.dll")
	hwnd, _, _ := kernel32.NewProc("GetConsoleWindow").Call()
	if hwnd != 0 {
		user32.NewProc("ShowWindow").Call(hwnd, 0)
	}
}

func ErrorBox(title, msg string) {
	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(msg)
	user32 := windows.NewLazySystemDLL("user32.dll")
	user32.NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), 0x10)
}

func OpenURL(url string) error {
	if url == "" {
		return fmt.Errorf("empty url")
	}
	verb, _ := syscall.UTF16PtrFromString("open")
	file, _ := syscall.UTF16PtrFromString(url)
	return windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL)
}
