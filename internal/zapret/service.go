package zapret

import (
	"fmt"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	ServiceName      = "zapret"
	ServiceDisplay   = "zapret"
	ServiceDesc      = "Zapret network acceleration service"
	StrategyRegValue = "zapret-discord-youtube"
)

type ServiceState struct {
	Installed bool   `json:"installed"`
	Status    string `json:"status"`
	Strategy  string `json:"strategy"`
	Winws     bool   `json:"winws"`
}

var (
	svcCacheMu sync.Mutex
	svcCache   ServiceState
	svcCacheAt time.Time
)

const svcCacheTTL = 400 * time.Millisecond

func QueryService() ServiceState {
	svcCacheMu.Lock()
	if time.Since(svcCacheAt) < svcCacheTTL {
		st := svcCache
		svcCacheMu.Unlock()
		return st
	}
	svcCacheMu.Unlock()

	st := queryService()

	svcCacheMu.Lock()
	svcCache = st
	svcCacheAt = time.Now()
	svcCacheMu.Unlock()
	return st
}

func InvalidateServiceCache() {
	svcCacheMu.Lock()
	svcCacheAt = time.Time{}
	svcCacheMu.Unlock()
}

func queryService() ServiceState {
	st := ServiceState{Status: "missing"}
	m, err := mgr.Connect()
	if err != nil {
		st.Winws = processRunning("winws.exe")
		return st
	}
	defer m.Disconnect()
	s, err := m.OpenService(ServiceName)
	if err != nil {
		st.Winws = processRunning("winws.exe")
		return st
	}
	defer s.Close()
	st.Installed = true
	st.Strategy = readStrategyName()
	status, err := s.Query()
	if err != nil {
		st.Status = "unknown"
		st.Winws = processRunning("winws.exe")
		return st
	}
	st.Status = stateName(status.State)
	if st.Status == "running" || st.Status == "starting" {
		st.Winws = true
		return st
	}
	st.Winws = processRunning("winws.exe")
	return st
}

func EnableService(root, strategyName string, games []GameStrategy) error {
	if !IsInstalled(root) {
		return fmt.Errorf("zapret is not installed in %s", root)
	}
	if err := EnsureUserLists(root); err != nil {
		return err
	}
	_ = EnsureTCPTimestamps()

	strats, err := ListStrategies(root)
	if err != nil {
		return err
	}
	var chosen *Strategy
	for i := range strats {
		if strings.EqualFold(strats[i].Name, strategyName) || strings.EqualFold(strats[i].FileName, strategyName) {
			chosen = &strats[i]
			break
		}
	}
	if chosen == nil {
		return fmt.Errorf("strategy not found: %s", strategyName)
	}
	gf := LoadGameFilter(root)
	args, err := ParseWinwsArgs(chosen.Path, root, gf)
	if err != nil {
		return err
	}
	if len(games) > 0 {
		args = InjectGameStrategies(args, root, games)
	}

	exe := WinwsPath(root)
	cmdline := serviceCmdLine(exe, args)

	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(ServiceName)
	if err != nil {
		return createAndStart(m, exe, args, chosen.Name)
	}
	closed := false
	defer func() {
		if !closed {
			s.Close()
		}
	}()

	status, qerr := s.Query()
	running := qerr == nil && (status.State == svc.Running || status.State == svc.StartPending)
	same := sameCmdLine(readCmdLineFingerprint(), cmdline)

	if running && same {
		_ = writeStrategyName(chosen.Name)
		InvalidateServiceCache()
		return nil
	}

	if !same {
		if err := updateServiceCmdLine(s, cmdline); err != nil {
			s.Close()
			closed = true
			_ = RemoveService(false)
			waitServiceDeleted(m, ServiceName, 2*time.Second)
			return createAndStart(m, exe, args, chosen.Name)
		}
	}

	if running {
		stopServiceHandle(s)
	}
	if err := writeStrategyName(chosen.Name); err != nil {
		return err
	}
	_ = writeCmdLineFingerprint(cmdline)
	err = s.Start()
	InvalidateServiceCache()
	return err
}

func createAndStart(m *mgr.Mgr, exe string, args []string, strategyName string) error {
	s, err := m.CreateService(ServiceName, exe, mgr.Config{
		DisplayName: ServiceDisplay,
		Description: ServiceDesc,
		StartType:   mgr.StartAutomatic,
	}, args...)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	defer s.Close()
	if err := writeStrategyName(strategyName); err != nil {
		return err
	}
	_ = writeCmdLineFingerprint(serviceCmdLine(exe, args))
	err = s.Start()
	InvalidateServiceCache()
	return err
}

func StopService() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(ServiceName)
	if err != nil {
		_ = killProcess("winws.exe")
		waitProcessGone("winws.exe", 2*time.Second)
		InvalidateServiceCache()
		return nil
	}
	defer s.Close()
	stopServiceHandle(s)
	InvalidateServiceCache()
	return nil
}

func stopServiceHandle(s *mgr.Service) {
	status, err := s.Control(svc.Stop)
	if err != nil {
		_ = killProcess("winws.exe")
		waitProcessGone("winws.exe", 2*time.Second)
		return
	}
	deadline := time.Now().Add(8 * time.Second)
	for status.State != svc.Stopped && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		status, err = s.Query()
		if err != nil {
			break
		}
	}
	if status.State != svc.Stopped {
		_ = killProcess("winws.exe")
	}
	waitProcessGone("winws.exe", 2*time.Second)
}

func serviceCmdLine(exepath string, args []string) string {
	s := syscall.EscapeArg(exepath)
	for _, v := range args {
		s += " " + syscall.EscapeArg(v)
	}
	return s
}

func sameCmdLine(a, b string) bool {
	return strings.EqualFold(compactCmdLine(a), compactCmdLine(b))
}

func compactCmdLine(s string) string {
	s = strings.ReplaceAll(s, `"`, "")
	return strings.Join(strings.Fields(s), " ")
}

func RemoveService(removeDriver bool) error {
	_ = StopService()
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	if s, err := m.OpenService(ServiceName); err == nil {
		_ = s.Delete()
		s.Close()
	}
	_ = killProcess("winws.exe")
	waitProcessGone("winws.exe", 2*time.Second)
	if removeDriver {
		for _, name := range []string{"WinDivert", "WinDivert14"} {
			if s, err := m.OpenService(name); err == nil {
				_, _ = s.Control(svc.Stop)
				deadline := time.Now().Add(1500 * time.Millisecond)
				for time.Now().Before(deadline) {
					st, qerr := s.Query()
					if qerr != nil || st.State == svc.Stopped {
						break
					}
					time.Sleep(50 * time.Millisecond)
				}
				_ = s.Delete()
				s.Close()
			}
		}
	}
	InvalidateServiceCache()
	return nil
}

func readStrategyName() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\`+ServiceName, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, err := k.GetStringValue(StrategyRegValue)
	if err != nil {
		return ""
	}
	return v
}

func writeStrategyName(name string) error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\`+ServiceName, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(StrategyRegValue, name)
}

const cmdlineRegValue = "zapret-cmdline"

func writeCmdLineFingerprint(cmdline string) error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\`+ServiceName, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(cmdlineRegValue, compactCmdLine(cmdline))
}

func readCmdLineFingerprint() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\`+ServiceName, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, err := k.GetStringValue(cmdlineRegValue)
	if err != nil {
		return ""
	}
	return v
}

func updateServiceCmdLine(s *mgr.Service, cmdline string) error {
	p, err := syscall.UTF16PtrFromString(cmdline)
	if err != nil {
		return err
	}
	return windows.ChangeServiceConfig(
		s.Handle,
		windows.SERVICE_NO_CHANGE,
		windows.SERVICE_NO_CHANGE,
		windows.SERVICE_NO_CHANGE,
		p,
		nil, nil, nil, nil, nil, nil,
	)
}

func stateName(s svc.State) string {
	switch s {
	case svc.Stopped:
		return "stopped"
	case svc.StartPending:
		return "starting"
	case svc.StopPending:
		return "stopping"
	case svc.Running:
		return "running"
	case svc.ContinuePending:
		return "continuing"
	case svc.PausePending:
		return "pausing"
	case svc.Paused:
		return "paused"
	default:
		return "unknown"
	}
}

func waitServiceDeleted(m *mgr.Mgr, name string, d time.Duration) {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		s, err := m.OpenService(name)
		if err != nil {
			return
		}
		s.Close()
		time.Sleep(40 * time.Millisecond)
	}
}

func waitProcessGone(name string, d time.Duration) {
	deadline := time.Now().Add(d)
	for processRunning(name) && time.Now().Before(deadline) {
		time.Sleep(40 * time.Millisecond)
	}
}

func processRunning(name string) bool {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(snap)
	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	if err := windows.Process32First(snap, &pe); err != nil {
		return false
	}
	for {
		if strings.EqualFold(windows.UTF16ToString(pe.ExeFile[:]), name) {
			return true
		}
		if err := windows.Process32Next(snap, &pe); err != nil {
			return false
		}
	}
}

func killProcess(name string) error {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snap)
	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	if err := windows.Process32First(snap, &pe); err != nil {
		return err
	}
	for {
		if strings.EqualFold(windows.UTF16ToString(pe.ExeFile[:]), name) {
			h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pe.ProcessID)
			if err == nil {
				_ = windows.TerminateProcess(h, 1)
				windows.CloseHandle(h)
			}
		}
		if err := windows.Process32Next(snap, &pe); err != nil {
			return nil
		}
	}
}
