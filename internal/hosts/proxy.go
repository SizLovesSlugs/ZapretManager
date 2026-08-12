package hosts

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

//go:embed proxies.json
var defaultProxiesJSON []byte

const (
	ProxyToken     = "PROXY"
	DefaultProxyIP = "95.182.120.241"
	proxyProbePort = "443"
	proxyProbeWait = 1500 * time.Millisecond
)

type Proxy struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	IP      string `json:"ip"`
	Default bool   `json:"default"`
}

type ProxyStatus struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	IP      string `json:"ip"`
	Default bool   `json:"default"`
	OK      bool   `json:"ok"`
	Latency int    `json:"latencyMs"` // -1 if unreachable
}

type proxiesFile struct {
	Proxies []Proxy `json:"proxies"`
}

var (
	proxiesMu    sync.RWMutex
	proxiesCache []Proxy
	proxiesOnce  sync.Once
)

func EnsureProxiesConfig() error {
	path := ProxiesConfigPath()
	if data, err := os.ReadFile(path); err == nil && bytes.Equal(data, defaultProxiesJSON) {
		return nil
	}
	if err := os.MkdirAll(dirOf(path), 0o755); err != nil {
		return err
	}
	// Keep shipped proxy list in sync with the app build.
	return os.WriteFile(path, defaultProxiesJSON, 0o644)
}

func Proxies() []Proxy {
	proxiesOnce.Do(func() {
		_ = EnsureProxiesConfig()
		list, err := loadProxies()
		if err != nil {
			list, _ = parseProxiesJSON(defaultProxiesJSON)
		}
		proxiesMu.Lock()
		proxiesCache = list
		proxiesMu.Unlock()
	})
	proxiesMu.RLock()
	defer proxiesMu.RUnlock()
	out := make([]Proxy, len(proxiesCache))
	copy(out, proxiesCache)
	return out
}

func ReloadProxies() error {
	proxiesMu.Lock()
	defer proxiesMu.Unlock()
	list, err := loadProxies()
	if err != nil {
		return err
	}
	proxiesCache = list
	return nil
}

func DefaultProxy() Proxy {
	list := Proxies()
	for _, p := range list {
		if p.Default {
			return p
		}
	}
	if len(list) > 0 {
		return list[0]
	}
	return Proxy{ID: "geohide-1", Name: "GeoHide 1", IP: DefaultProxyIP, Default: true}
}

func ProxyByIP(ip string) (Proxy, bool) {
	ip = strings.TrimSpace(ip)
	for _, p := range Proxies() {
		if p.IP == ip {
			return p, true
		}
	}
	return Proxy{}, false
}

func NormalizeProxyIP(ip string) string {
	ip = strings.TrimSpace(ip)
	if _, ok := ProxyByIP(ip); ok {
		return ip
	}
	return DefaultProxy().IP
}

func PingProxies() []ProxyStatus {
	list := Proxies()
	out := make([]ProxyStatus, len(list))
	var wg sync.WaitGroup
	for i, p := range list {
		wg.Add(1)
		go func(i int, p Proxy) {
			defer wg.Done()
			ok, ms := ProbeTCP(p.IP, proxyProbePort, proxyProbeWait)
			out[i] = ProxyStatus{
				ID:      p.ID,
				Name:    p.Name,
				IP:      p.IP,
				Default: p.Default,
				OK:      ok,
				Latency: ms,
			}
		}(i, p)
	}
	wg.Wait()
	return out
}

func ProbeTCP(ip, port string, timeout time.Duration) (bool, int) {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, port), timeout)
	ms := int(time.Since(start).Milliseconds())
	if err != nil {
		return false, -1
	}
	_ = conn.Close()
	return true, ms
}

func loadProxies() ([]Proxy, error) {
	_ = EnsureProxiesConfig()
	data, err := os.ReadFile(ProxiesConfigPath())
	if err != nil {
		data = defaultProxiesJSON
	}
	list, err := parseProxiesJSON(data)
	if err != nil {
		list, err = parseProxiesJSON(defaultProxiesJSON)
		if err != nil {
			return nil, err
		}
	}
	return list, nil
}

func parseProxiesJSON(data []byte) ([]Proxy, error) {
	var cfg proxiesFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("proxies.json: %w", err)
	}
	out := make([]Proxy, 0, len(cfg.Proxies))
	seen := map[string]bool{}
	for _, p := range cfg.Proxies {
		ip := strings.TrimSpace(p.IP)
		name := strings.TrimSpace(p.Name)
		id := strings.TrimSpace(p.ID)
		if ip == "" || name == "" || net.ParseIP(ip) == nil {
			continue
		}
		if id == "" {
			id = ip
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, Proxy{ID: id, Name: name, IP: ip, Default: p.Default})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("proxies.json: нет прокси")
	}
	return out, nil
}

func resolveEntryIP(ip, proxyIP string) string {
	if strings.EqualFold(ip, ProxyToken) {
		return proxyIP
	}
	return ip
}
