package hosts

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
)

//go:embed hosts.json
var defaultHostsJSON []byte

const (
	IDTelegram          = "telegram"
	IDInstagram         = "instagram"
	IDAI                = "ai"
	IDTwitch            = "twitch"
	IDGithub            = "github"
	IDGithubusercontent = "githubusercontent"
	IDSupercell         = "supercell"
	IDSpotify           = "spotify"
	IDNalog             = "nalog"
	IDRutor             = "rutor"
	IDNtc               = "ntc"
	IDLibrusec          = "librusec"
)

type Entry struct {
	IP    string
	Hosts []string
}

type Profile struct {
	ID        string
	Title     string
	DefaultOn bool
	Comment   string
	Entries   []Entry
}

type fileProfile struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	DefaultOn bool     `json:"defaultOn"`
	Hosts     []string `json:"hosts"`
}

type fileConfig struct {
	Profiles []fileProfile `json:"profiles"`
}

var (
	profilesMu   sync.RWMutex
	cached       []Profile
	profilesOnce sync.Once
)

func EnsureConfig() error {
	// Always refresh shipped hosts.json (same approach as proxies.json).
	return SyncConfigFile()
}

// SyncConfigFile overwrites DataRoot hosts.json with the embedded defaults.
func SyncConfigFile() error {
	path := ConfigPath()
	if err := os.MkdirAll(dirOf(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, defaultHostsJSON, 0o644)
}

func Reload() error {
	profilesMu.Lock()
	defer profilesMu.Unlock()
	_ = SyncConfigFile()
	list, err := loadProfiles()
	if err != nil {
		return err
	}
	cached = list
	return nil
}

func Profiles() []Profile {
	profilesOnce.Do(func() {
		_ = EnsureConfig()
		list, err := loadProfiles()
		if err != nil {
			list, _ = parseProfilesJSON(defaultHostsJSON)
		}
		profilesMu.Lock()
		cached = list
		profilesMu.Unlock()
	})
	profilesMu.RLock()
	defer profilesMu.RUnlock()
	out := make([]Profile, len(cached))
	copy(out, cached)
	return out
}

func ExtraProfiles() []Profile {
	all := Profiles()
	out := make([]Profile, 0, len(all))
	for _, p := range all {
		if p.ID == IDTelegram {
			continue
		}
		out = append(out, p)
	}
	return out
}

func ProfileByID(id string) (Profile, bool) {
	for _, p := range Profiles() {
		if p.ID == id {
			return p, true
		}
	}
	return Profile{}, false
}

func DefaultOn(id string) bool {
	p, ok := ProfileByID(id)
	if !ok {
		return false
	}
	return p.DefaultOn
}

func loadProfiles() ([]Profile, error) {
	_ = EnsureConfig()
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		data = defaultHostsJSON
	}
	list, err := parseProfilesJSON(data)
	if err != nil {
		// broken user file — fall back to embedded defaults
		list, err = parseProfilesJSON(defaultHostsJSON)
		if err != nil {
			return nil, err
		}
	}
	return list, nil
}

func parseProfilesJSON(data []byte) ([]Profile, error) {
	var cfg fileConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("hosts.json: %w", err)
	}
	if len(cfg.Profiles) == 0 {
		return nil, fmt.Errorf("hosts.json: нет профилей")
	}
	out := make([]Profile, 0, len(cfg.Profiles))
	seen := map[string]bool{}
	for _, raw := range cfg.Profiles {
		id := strings.TrimSpace(raw.ID)
		title := strings.TrimSpace(raw.Title)
		if id == "" || title == "" {
			continue
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		entries := parseHostLines(raw.Hosts)
		if len(entries) == 0 {
			continue
		}
		out = append(out, Profile{
			ID:        id,
			Title:     title,
			DefaultOn: raw.DefaultOn,
			Comment:   title,
			Entries:   entries,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("hosts.json: нет валидных профилей")
	}
	return out, nil
}

func parseHostLines(lines []string) []Entry {
	var out []Entry
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ip := fields[0]
		if !strings.EqualFold(ip, ProxyToken) && net.ParseIP(ip) == nil {
			continue
		}
		if strings.EqualFold(ip, ProxyToken) {
			ip = ProxyToken
		}
		out = append(out, Entry{
			IP:    ip,
			Hosts: append([]string(nil), fields[1:]...),
		})
	}
	return out
}
