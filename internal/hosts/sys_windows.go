//go:build windows

package hosts

import (
	"os/exec"
	"sync/atomic"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

func hiddenWindow() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}

const (
	ipv4PreferMask   = uint32(0x20)
	tcpip6Parameters = `SYSTEM\CurrentControlSet\Services\Tcpip6\Parameters`
)

var lastIPv4Prefer atomic.Int32 // 0 unknown, 1 on, 2 off

func setPreferIPv4(on bool) {
	setPreferIPv4Registry(on)
	want := int32(2)
	if on {
		want = 1
	}
	if lastIPv4Prefer.Load() == want {
		return
	}
	setPreferIPv4PrefixPolicy(on)
	lastIPv4Prefer.Store(want)
}

func setPreferIPv4Registry(on bool) {
	key, _, err := registry.CreateKey(registry.LOCAL_MACHINE, tcpip6Parameters, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return
	}
	defer key.Close()

	var cur uint32
	if v, _, err := key.GetIntegerValue("DisabledComponents"); err == nil {
		cur = uint32(v)
	}
	next := cur
	if on {
		next |= ipv4PreferMask
	} else {
		next &^= ipv4PreferMask
	}
	if next == cur {
		return
	}
	_ = key.SetDWordValue("DisabledComponents", next)
}

func setPreferIPv4PrefixPolicy(on bool) {
	v6prio, v4prio := "40", "35"
	if on {
		v6prio, v4prio = "10", "50"
	}
	runNetsh("interface", "ipv6", "set", "prefixpolicy", "::/0", v6prio, "1")
	runNetsh("interface", "ipv6", "set", "prefixpolicy", "::ffff:0:0/96", v4prio, "4")
}

func runNetsh(args ...string) {
	cmd := exec.Command("netsh", args...)
	cmd.SysProcAttr = hiddenWindow()
	_ = cmd.Run()
}
