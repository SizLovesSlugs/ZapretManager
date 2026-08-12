//go:build !windows

package hosts

import "syscall"

func hiddenWindow() *syscall.SysProcAttr {
	return nil
}

func setPreferIPv4(on bool) {}
