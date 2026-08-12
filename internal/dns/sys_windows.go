//go:build windows

package dns

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"zapret-manager/internal/winutil"
)

const createNoWindow = 0x08000000

func hiddenWindow() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

type liveAdapter struct {
	Alias string
	Index uint32
	GUID  string
}

func readAdapters() ([]adapterBackup, error) {
	live, err := listUpAdapters()
	if err != nil {
		return nil, err
	}
	out := make([]adapterBackup, 0, len(live))
	for _, a := range live {
		dhcp, servers := readAdapterDNS(a.GUID)
		out = append(out, adapterBackup{
			Alias:   a.Alias,
			Index:   int(a.Index),
			DHCP:    dhcp,
			Servers: servers,
		})
	}
	return out, nil
}

func setStaticDNS(servers []string) error {
	if len(servers) == 0 {
		return fmt.Errorf("empty dns servers")
	}
	live, err := listUpAdapters()
	if err != nil {
		return err
	}
	if len(live) == 0 {
		return fmt.Errorf("не найдены активные сетевые адаптеры")
	}
	var errs []string
	for _, a := range live {
		dhcp, cur := readAdapterDNS(a.GUID)
		if !dhcp && sameDNS(cur, servers) {
			continue
		}
		if err := setAdapterDNS(a.Alias, servers); err != nil {
			errs = append(errs, a.Alias+": "+err.Error())
			continue
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func restoreBackup(bak backupFile) error {
	var errs []string
	for _, a := range bak.Adapters {
		if strings.TrimSpace(a.Alias) == "" {
			continue
		}
		var err error
		if a.DHCP || len(a.Servers) == 0 {
			err = runNetsh("interface", "ipv4", "set", "dnsservers", "name="+a.Alias, "source=dhcp")
		} else {
			err = setAdapterDNS(a.Alias, a.Servers)
		}
		if err != nil {
			errs = append(errs, a.Alias+": "+err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("восстановление DNS: %s", strings.Join(errs, "; "))
	}
	return nil
}

func setAdapterDNS(alias string, servers []string) error {
	if err := runNetsh("interface", "ipv4", "set", "dnsservers", "name="+alias, "source=static", "address="+servers[0], "register=none", "validate=no"); err != nil {
		return err
	}
	for i, s := range servers[1:] {
		if err := runNetsh("interface", "ipv4", "add", "dnsservers", "name="+alias, "address="+s, "index="+strconv.Itoa(i+2), "validate=no"); err != nil {
			return err
		}
	}
	return nil
}

func flushDNS() {
	winutil.FlushResolverCache()
}

func runNetsh(args ...string) error {
	cmd := exec.Command("netsh", args...)
	cmd.SysProcAttr = hiddenWindow()
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func listUpAdapters() ([]liveAdapter, error) {
	var size uint32
	flags := uint32(windows.GAA_FLAG_SKIP_ANYCAST | windows.GAA_FLAG_SKIP_MULTICAST)
	err := windows.GetAdaptersAddresses(windows.AF_INET, flags, 0, nil, &size)
	if err != nil && err != windows.ERROR_BUFFER_OVERFLOW {
		return nil, err
	}
	if size == 0 {
		return nil, nil
	}
	buf := make([]byte, size)
	addr := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0]))
	if err := windows.GetAdaptersAddresses(windows.AF_INET, flags, 0, addr, &size); err != nil {
		return nil, err
	}
	var out []liveAdapter
	for ; addr != nil; addr = addr.Next {
		if addr.IfType == windows.IF_TYPE_SOFTWARE_LOOPBACK {
			continue
		}
		if addr.OperStatus != windows.IfOperStatusUp {
			continue
		}
		name := windows.UTF16PtrToString(addr.FriendlyName)
		guid := windows.BytePtrToString(addr.AdapterName)
		if name == "" {
			continue
		}
		out = append(out, liveAdapter{Alias: name, Index: addr.IfIndex, GUID: guid})
	}
	return out, nil
}

func readAdapterDNS(guid string) (dhcp bool, servers []string) {
	guid = strings.TrimSpace(guid)
	if guid == "" {
		return true, nil
	}
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces\`+guid, registry.QUERY_VALUE)
	if err != nil {
		return true, nil
	}
	defer k.Close()
	if ns, _, err := k.GetStringValue("NameServer"); err == nil {
		if servers = splitDNS(ns); len(servers) > 0 {
			return false, servers
		}
	}
	if ns, _, err := k.GetStringValue("DhcpNameServer"); err == nil {
		return true, splitDNS(ns)
	}
	return true, nil
}

func splitDNS(s string) []string {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", " ")
	fields := strings.Fields(s)
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

func sameDNS(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
