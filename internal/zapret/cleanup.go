package zapret

import (
	"strings"
	"time"

	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// Known services that conflict with zapret / hold WinDivert.
var conflictServiceNames = []string{
	"zapret",
	"GoodbyeDPI",
	"WinDivert",
	"WinDivert14",
}

// CleanupConflicts stops/removes foreign zapret-like services and WinDivert
// drivers so the first Start after launch does not fight Flowseal/other packs.
func CleanupConflicts() ([]string, error) {
	m, err := mgr.Connect()
	if err != nil {
		return nil, err
	}
	defer m.Disconnect()

	seen := map[string]bool{}
	var removed []string
	note := func(name string) {
		key := strings.ToLower(name)
		if seen[key] {
			return
		}
		seen[key] = true
		removed = append(removed, name)
	}

	for _, name := range conflictServiceNames {
		if stopAndDeleteService(m, name) {
			note(name)
		}
	}
	for _, name := range winwsServiceNamesFromRegistry() {
		if seen[strings.ToLower(name)] {
			continue
		}
		if stopAndDeleteService(m, name) {
			note(name)
		}
	}

	_ = killProcess("winws.exe")
	_ = killProcess("goodbyedpi.exe")
	waitProcessGone("winws.exe", 2*time.Second)
	InvalidateServiceCache()
	return removed, nil
}

func winwsServiceNamesFromRegistry() []string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services`, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return nil
	}
	defer k.Close()
	names, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return nil
	}
	var out []string
	for _, name := range names {
		sk, err := registry.OpenKey(k, name, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		img, _, err := sk.GetStringValue("ImagePath")
		sk.Close()
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(img), "winws.exe") {
			out = append(out, name)
		}
	}
	return out
}

func stopAndDeleteService(m *mgr.Mgr, name string) bool {
	s, err := m.OpenService(name)
	if err != nil {
		return false
	}
	defer s.Close()

	status, err := s.Query()
	if err == nil && status.State != svc.Stopped {
		_, _ = s.Control(svc.Stop)
		deadline := time.Now().Add(4 * time.Second)
		for time.Now().Before(deadline) {
			status, err = s.Query()
			if err != nil || status.State == svc.Stopped {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	_ = s.Delete()
	return true
}
