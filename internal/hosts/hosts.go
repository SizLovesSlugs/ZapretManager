package hosts

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"zapret-manager/internal/winutil"
)

func HostsPath() string {
	if runtime.GOOS == "windows" {
		root := os.Getenv("SystemRoot")
		if root == "" {
			root = `C:\Windows`
		}
		return filepath.Join(root, "System32", "drivers", "etc", "hosts")
	}
	return "/etc/hosts"
}

func beginMarker(id string) string {
	if id == IDTelegram {
		return "# BEGIN Zapret Manager Telegram Web"
	}
	return "# BEGIN Zapret Manager boost:" + id
}

func endMarker(id string) string {
	if id == IDTelegram {
		return "# END Zapret Manager Telegram Web"
	}
	return "# END Zapret Manager boost:" + id
}

func ApplyTelegramWebBoost(enabled bool) error {
	return ApplyProfile(IDTelegram, enabled, DefaultProxyIP)
}

func ApplyProfile(id string, enabled bool, proxyIP string) error {
	p, ok := ProfileByID(id)
	if !ok {
		return fmt.Errorf("неизвестный профиль: %s", id)
	}
	proxyIP = NormalizeProxyIP(proxyIP)
	path := HostsPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("hosts: %w", err)
	}
	next := ApplyProfileBlock(string(raw), p, enabled, proxyIP)
	if next == string(raw) {
		syncIPv4Preference(next)
		return nil
	}
	if err := os.WriteFile(path, []byte(next), 0o644); err != nil {
		return fmt.Errorf("запись hosts: %w", err)
	}
	flushDNS()
	syncIPv4Preference(next)
	return nil
}

func ApplyAll(enabled map[string]bool, proxyIP string) error {
	proxyIP = NormalizeProxyIP(proxyIP)
	path := HostsPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("hosts: %w", err)
	}
	content := string(raw)
	for _, p := range Profiles() {
		on := false
		if enabled != nil {
			if v, ok := enabled[p.ID]; ok {
				on = v
			} else {
				on = p.DefaultOn
			}
		} else {
			on = p.DefaultOn
		}
		content = ApplyProfileBlock(content, p, on, proxyIP)
	}
	if content != string(raw) {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("запись hosts: %w", err)
		}
	}
	flushDNS()
	syncIPv4Preference(content)
	return nil
}

func ClearAll() error {
	enabled := map[string]bool{}
	for _, p := range Profiles() {
		enabled[p.ID] = false
	}
	return ApplyAll(enabled, DefaultProxyIP)
}

// EnabledDomains returns hostnames from profiles that are currently enabled.
func EnabledDomains(enabled map[string]bool) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range Profiles() {
		on := false
		if enabled != nil {
			if v, ok := enabled[p.ID]; ok {
				on = v
			} else {
				on = p.DefaultOn
			}
		} else {
			on = p.DefaultOn
		}
		if !on {
			continue
		}
		for _, entry := range p.Entries {
			for _, host := range entry.Hosts {
				key := strings.ToLower(strings.TrimSpace(host))
				if key == "" || seen[key] {
					continue
				}
				seen[key] = true
				out = append(out, key)
			}
		}
	}
	return out
}

func HasProfileBlock(content, id string) bool {
	return strings.Contains(content, beginMarker(id)) && strings.Contains(content, endMarker(id))
}

func ApplyProfileBlock(content string, p Profile, enabled bool, proxyIP string) string {
	nl := detectNewline(content)
	cleaned := stripProfileBlock(content, p.ID)
	cleaned = strings.TrimRight(cleaned, " \t\r\n")
	if !enabled {
		if cleaned == "" {
			return ""
		}
		return cleaned + nl
	}
	block := buildProfileBlock(p, nl, NormalizeProxyIP(proxyIP))
	if block == "" {
		if cleaned == "" {
			return ""
		}
		return cleaned + nl
	}
	if cleaned == "" {
		return block
	}
	return cleaned + nl + nl + block
}

func stripProfileBlock(content, id string) string {
	begin := beginMarker(id)
	end := endMarker(id)
	for {
		start := strings.Index(content, begin)
		if start < 0 {
			return content
		}
		endIdx := strings.Index(content[start:], end)
		if endIdx < 0 {
			return strings.TrimRight(content[:start], " \t\r\n")
		}
		endPos := start + endIdx + len(end)
		for endPos < len(content) && (content[endPos] == '\n' || content[endPos] == '\r') {
			endPos++
		}
		before := strings.TrimRight(content[:start], " \t\r\n")
		after := strings.TrimLeft(content[endPos:], " \t")
		if after != "" && (strings.HasPrefix(after, "\r\n") || strings.HasPrefix(after, "\n")) {
			after = strings.TrimLeft(after, "\r\n")
		}
		if before == "" {
			content = after
			continue
		}
		if after == "" {
			content = before
			continue
		}
		content = before + detectNewline(content) + after
	}
}

func buildProfileBlock(p Profile, nl, proxyIP string) string {
	// Windows hosts: first match wins per hostname. Keep first IP only.
	seen := map[string]bool{}
	var lines []string
	for _, entry := range p.Entries {
		ip := resolveEntryIP(entry.IP, proxyIP)
		var names []string
		for _, host := range entry.Hosts {
			key := strings.ToLower(host)
			if seen[key] {
				continue
			}
			seen[key] = true
			names = append(names, host)
		}
		if len(names) == 0 {
			continue
		}
		line := ip
		for _, host := range names {
			line += " " + host
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(beginMarker(p.ID))
	b.WriteString(nl)
	if p.Comment != "" {
		b.WriteString("# ")
		b.WriteString(p.Comment)
		b.WriteString(nl)
	}
	for _, line := range lines {
		b.WriteString(line)
		b.WriteString(nl)
	}
	b.WriteString(endMarker(p.ID))
	b.WriteString(nl)
	return b.String()
}

func managedHostsActive(content string) bool {
	for _, p := range Profiles() {
		if HasProfileBlock(content, p.ID) {
			return true
		}
	}
	return false
}

func syncIPv4Preference(hostsContent string) {
	setPreferIPv4(managedHostsActive(hostsContent))
}

func detectNewline(content string) string {
	if strings.Contains(content, "\r\n") {
		return "\r\n"
	}
	return "\n"
}

func flushDNS() {
	winutil.FlushResolverCache()
}

// Compatibility helpers used by older tests / callers.
func ApplyTelegramBlock(content string, enabled bool) string {
	p, _ := ProfileByID(IDTelegram)
	return ApplyProfileBlock(content, p, enabled, DefaultProxyIP)
}

func HasTelegramBlock(content string) bool {
	return HasProfileBlock(content, IDTelegram)
}

func TelegramDomains() []string {
	p, _ := ProfileByID(IDTelegram)
	var out []string
	seen := map[string]bool{}
	for _, entry := range p.Entries {
		for _, h := range entry.Hosts {
			if seen[h] {
				continue
			}
			seen[h] = true
			out = append(out, h)
		}
	}
	return out
}
