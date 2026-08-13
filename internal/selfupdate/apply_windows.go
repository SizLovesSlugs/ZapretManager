package selfupdate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

const createNoWindow = 0x08000000

func Apply(newPath string) error {
	exe, err := currentExe()
	if err != nil {
		return err
	}
	if newPath == "" {
		newPath = exe + ".new"
	}
	if _, err := os.Stat(newPath); err != nil {
		return err
	}
	if err := verifyPE(newPath); err != nil {
		return err
	}

	same := strings.EqualFold(filepath.Clean(newPath), filepath.Clean(exe))
	oldPath := exe + ".old"
	_ = os.Remove(oldPath)

	if same {
		if err := os.Rename(exe, oldPath); err != nil {
			return applyViaCmd(exe, newPath)
		}
		if err := os.Rename(newPath, exe); err != nil {
			_ = os.Rename(oldPath, exe)
			return applyViaCmd(exe, newPath)
		}
		if err := startDetached(exe); err != nil {
			_ = os.Rename(exe, newPath)
			_ = os.Rename(oldPath, exe)
			return err
		}
		os.Exit(0)
		return nil
	}

	if err := os.Rename(exe, oldPath); err != nil {
		return applyViaCmd(exe, newPath)
	}
	if err := startDetached(newPath); err != nil {
		_ = os.Rename(oldPath, exe)
		return err
	}
	os.Exit(0)
	return nil
}

func startDetached(exe string) error {
	cmd := exec.Command(exe)
	cmd.Dir = filepath.Dir(exe)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &windows.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = cmd.Process.Release()
	return nil
}

func applyViaCmd(oldExe, newPath string) error {
	bat := filepath.Join(os.TempDir(), "zapret-manager-update.cmd")
	script := "@echo off\r\nsetlocal\r\nset \"TARGET=%~1\"\r\nset \"NEW=%~2\"\r\nset /a n=0\r\n:wait\r\nset /a n+=1\r\nif %n% GEQ 40 goto fail\r\nping -n 2 127.0.0.1 >nul\r\ndel /f \"%TARGET%\" >nul 2>&1\r\nif exist \"%TARGET%\" goto wait\r\nstart \"\" \"%NEW%\"\r\ndel \"%~f0\"\r\nexit /b 0\r\n:fail\r\nstart \"\" \"%NEW%\"\r\ndel \"%~f0\"\r\nexit /b 1\r\n"
	if err := os.WriteFile(bat, []byte(script), 0o644); err != nil {
		return err
	}
	cmd := exec.Command(bat, oldExe, newPath)
	cmd.SysProcAttr = &windows.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = cmd.Process.Release()
	os.Exit(0)
	return nil
}
