package zapret

import (
	"fmt"
	"os"
	"path/filepath"
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
		return fmt.Errorf("установка: zapret не найден в %s", root)
	}
	if err := EnsureUserLists(root); err != nil {
		return fmt.Errorf("списки: %w", err)
	}
	if err := EnsureTCPTimestamps(); err != nil {
		// best-effort: preflight already logs this
		_ = err
	}

	strats, err := ListStrategies(root)
	if err != nil {
		return fmt.Errorf("список стратегий: %w", err)
	}
	var chosen *Strategy
	for i := range strats {
		if strings.EqualFold(strats[i].Name, strategyName) || strings.EqualFold(strats[i].FileName, strategyName) {
			chosen = &strats[i]
			break
		}
	}
	if chosen == nil {
		return fmt.Errorf("стратегия не найдена: %s", strategyName)
	}
	gf := LoadGameFilter(root)
	args, err := ParseWinwsArgs(chosen.Path, root, gf)
	if err != nil {
		return fmt.Errorf("разбор %s: %w", chosen.FileName, err)
	}
	if len(games) > 0 {
		if err := checkGameBins(root, games); err != nil {
			return err
		}
		args = InjectGameStrategies(args, root, games)
	}

	if err := waitWinDivertIdle(8 * time.Second); err != nil {
		return err
	}

	exe := WinwsPath(root)
	if _, err := os.Stat(exe); err != nil {
		return fmt.Errorf("winws: %w", err)
	}
	cmdline := serviceCmdLine(exe, args)

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("диспетчер служб: %w", err)
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
		if err := confirmServiceStayedUp(s); err != nil {
			return fmt.Errorf("%w; %s", err, summarizeWinwsArgs(args))
		}
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
		return fmt.Errorf("запись стратегии: %w", err)
	}
	_ = writeCmdLineFingerprint(cmdline)
	if err := s.Start(); err != nil {
		InvalidateServiceCache()
		return fmt.Errorf("запуск службы: %w", err)
	}
	if err := confirmServiceStayedUp(s); err != nil {
		InvalidateServiceCache()
		return fmt.Errorf("%w; %s", err, summarizeWinwsArgs(args))
	}
	InvalidateServiceCache()
	return nil
}

func createAndStart(m *mgr.Mgr, exe string, args []string, strategyName string) error {
	s, err := m.CreateService(ServiceName, exe, mgr.Config{
		DisplayName: ServiceDisplay,
		Description: ServiceDesc,
		StartType:   mgr.StartAutomatic,
	}, args...)
	if err != nil {
		return fmt.Errorf("создание службы: %w", err)
	}
	defer s.Close()
	if err := writeStrategyName(strategyName); err != nil {
		return fmt.Errorf("запись стратегии: %w", err)
	}
	_ = writeCmdLineFingerprint(serviceCmdLine(exe, args))
	if err := s.Start(); err != nil {
		InvalidateServiceCache()
		return fmt.Errorf("запуск службы: %w", err)
	}
	if err := confirmServiceStayedUp(s); err != nil {
		InvalidateServiceCache()
		return fmt.Errorf("%w; %s", err, summarizeWinwsArgs(args))
	}
	InvalidateServiceCache()
	return nil
}

func checkGameBins(root string, games []GameStrategy) error {
	seen := map[string]bool{}
	for _, g := range mergeGameGroups(games) {
		if seen[g.FakeUDP] {
			continue
		}
		seen[g.FakeUDP] = true
		path := filepath.Join(BinDir(root), g.FakeUDP)
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("игровая стратегия %s: нет файла %s", g.Name, g.FakeUDP)
		}
	}
	return nil
}

func summarizeWinwsArgs(args []string) string {
	var wfUDP, filters []string
	news := 0
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--wf-udp="):
			wfUDP = append(wfUDP, strings.TrimPrefix(a, "--wf-udp="))
		case strings.HasPrefix(a, "--filter-udp="):
			filters = append(filters, strings.TrimPrefix(a, "--filter-udp="))
		case a == "--new":
			news++
		}
	}
	return fmt.Sprintf("wf-udp=%s filter-udp=%s --new=%d", strings.Join(wfUDP, "|"), strings.Join(filters, "|"), news)
}

func confirmServiceStayedUp(s *mgr.Service) error {
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		st, err := s.Query()
		if err != nil {
			return fmt.Errorf("проверка службы: %w", err)
		}
		switch st.State {
		case svc.Running:
			time.Sleep(700 * time.Millisecond)
			st, err = s.Query()
			if err != nil {
				return fmt.Errorf("проверка службы: %w", err)
			}
			if st.State != svc.Running {
				if err := winDivertStuckError(); err != nil {
					return err
				}
				return fmt.Errorf("winws завершился сразу после запуска (статус %s). Часто это слишком широкий UDP-фильтр WinDivert или битая командная строка", stateName(st.State))
			}
			if !processRunning("winws.exe") {
				return fmt.Errorf("служба числится running, но процесс winws.exe уже нет")
			}
			return nil
		case svc.StartPending:
			time.Sleep(100 * time.Millisecond)
		default:
			time.Sleep(100 * time.Millisecond)
			if time.Now().After(deadline.Add(-500 * time.Millisecond)) {
				return fmt.Errorf("winws не запустился (статус %s)", stateName(st.State))
			}
		}
	}
	st, err := s.Query()
	if err != nil {
		return fmt.Errorf("winws не запустился за 3с: %w", err)
	}
	if st.State != svc.Running {
		return fmt.Errorf("winws не запустился за 3с (статус %s)", stateName(st.State))
	}
	return nil
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
		for _, name := range winDivertNames {
			_ = stopAndDeleteService(m, name)
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
