package zapret

import (
	"os/exec"
	"strings"
	"sync/atomic"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

var tcpTimestampsReady atomic.Bool

// EnsureTCPTimestamps enables global TCP timestamps when they are not already on.
func EnsureTCPTimestamps() error {
	if tcpTimestampsReady.Load() {
		return nil
	}
	if tcpTimestampsEnabled() {
		tcpTimestampsReady.Store(true)
		return nil
	}
	if err := enableTCPTimestamps(); err != nil {
		return err
	}
	tcpTimestampsReady.Store(true)
	return nil
}

func enableTCPTimestamps() error {
	cmd := exec.Command("netsh", "interface", "tcp", "set", "global", "timestamps=enabled")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	return cmd.Run()
}

func tcpTimestampsEnabled() bool {
	if enabled, ok := tcpTimestampsFromRegistry(); ok {
		return enabled
	}
	cmd := exec.Command("netsh", "interface", "tcp", "show", "global")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return parseTCPTimestampsEnabled(string(out))
}

// Tcp1323Opts: 0 none, 1 window scale, 2 timestamps, 3 both.
func tcpTimestampsFromRegistry() (enabled bool, ok bool) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\Tcpip\Parameters`, registry.QUERY_VALUE)
	if err != nil {
		return false, false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("Tcp1323Opts")
	if err != nil {
		return false, false
	}
	return v == 2 || v == 3, true
}

func parseTCPTimestampsEnabled(out string) bool {
	for _, line := range strings.Split(out, "\n") {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		low := strings.ToLower(l)
		isTS := strings.Contains(low, "timestamp") ||
			(strings.Contains(low, "метк") && strings.Contains(low, "врем"))
		if !isTS {
			continue
		}
		if strings.Contains(low, "disabled") || strings.Contains(low, "отключ") {
			return false
		}
		if strings.Contains(low, "enabled") || strings.Contains(low, "включ") {
			return true
		}
		return false
	}
	return false
}
