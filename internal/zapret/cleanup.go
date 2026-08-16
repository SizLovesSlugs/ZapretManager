package zapret

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// User-mode services that conflict with our zapret. Kernel WinDivert is
// never deleted here: Delete() on a still-stopping driver leaves it in
// STOP_PENDING forever, and winws then dies with "device does not exist".
var conflictServiceNames = []string{
	"zapret",
	"GoodbyeDPI",
}

var winDivertNames = []string{"WinDivert", "WinDivert14"}

// CleanupConflicts stops/removes foreign zapret-like user-mode services
// so the first Start after launch does not fight Flowseal/other packs.
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

func winDivertStuckError() error {
	m, err := mgr.Connect()
	if err != nil {
		return nil
	}
	defer m.Disconnect()
	for _, name := range winDivertNames {
		s, err := m.OpenService(name)
		if err != nil {
			continue
		}
		st, qerr := s.Query()
		s.Close()
		if qerr != nil {
			continue
		}
		if st.State == svc.StopPending {
			return fmt.Errorf("драйвер WinDivert завис (STOP_PENDING). Перезагрузите компьютер один раз, затем включите снова")
		}
	}
	return nil
}

func waitWinDivertIdle(timeout time.Duration) error {
	if err := winDivertStuckError(); err == nil {
		return nil
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(400 * time.Millisecond)
		if err := winDivertStuckError(); err == nil {
			return nil
		}
	}
	return winDivertStuckError()
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
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) {
			status, err = s.Query()
			if err != nil || status.State == svc.Stopped {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	if isWinDivertName(name) {
		if err != nil || status.State != svc.Stopped {
			return false
		}
	}
	_ = s.Delete()
	return true
}

func isWinDivertName(name string) bool {
	for _, n := range winDivertNames {
		if strings.EqualFold(n, name) {
			return true
		}
	}
	return false
}
