package dns

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"zapret-manager/internal/zapret"
)

const (
	IDCloudflare = "cloudflare"
	IDGoogle     = "google"
	IDYandex     = "yandex"
	IDSystem     = "system"
)

type Profile struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Default bool     `json:"default"`
	Servers []string `json:"servers,omitempty"`
}

type adapterBackup struct {
	Alias   string   `json:"alias"`
	Index   int      `json:"index"`
	DHCP    bool     `json:"dhcp"`
	Servers []string `json:"servers,omitempty"`
}

type backupFile struct {
	Adapters []adapterBackup `json:"adapters"`
}

var allProfiles = []Profile{
	{ID: IDCloudflare, Title: "Cloudflare", Default: true, Servers: []string{"1.1.1.1", "1.0.0.1"}},
	{ID: IDGoogle, Title: "Google", Servers: []string{"8.8.8.8", "8.8.4.4"}},
	{ID: IDYandex, Title: "Yandex", Servers: []string{"77.88.8.8", "77.88.8.1"}},
	{ID: IDSystem, Title: "Системные DNS"},
}

func Profiles() []Profile {
	return allProfiles
}

func NormalizeID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	for _, p := range Profiles() {
		if p.ID == id {
			return p.ID
		}
	}
	return IDCloudflare
}

func ProfileByID(id string) (Profile, bool) {
	id = NormalizeID(id)
	for _, p := range Profiles() {
		if p.ID == id {
			return p, true
		}
	}
	return Profile{}, false
}

func BackupPath() string {
	return filepath.Join(zapret.DataRoot(), "dns-backup.json")
}

func HasBackup() bool {
	_, err := os.Stat(BackupPath())
	return err == nil
}

// SyncForService applies or restores DNS depending on whether zapret is active
// and which profile is selected.
func SyncForService(active bool, profileID string) error {
	profileID = NormalizeID(profileID)
	if !active {
		return Restore()
	}
	return Apply(profileID)
}

// Apply sets the selected DNS profile. System restores the original backup.
// Non-system profiles snapshot current DNS once, then set static servers.
func Apply(profileID string) error {
	profileID = NormalizeID(profileID)
	if profileID == IDSystem {
		return Restore()
	}
	p, ok := ProfileByID(profileID)
	if !ok || len(p.Servers) == 0 {
		return fmt.Errorf("неизвестный DNS профиль")
	}
	if !HasBackup() {
		if err := saveBackup(); err != nil {
			return err
		}
	}
	if err := setStaticDNS(p.Servers); err != nil {
		return err
	}
	flushDNS()
	return nil
}

// Restore returns adapters to the DNS settings captured before our changes.
func Restore() error {
	path := BackupPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var bak backupFile
	if err := json.Unmarshal(data, &bak); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("dns backup: %w", err)
	}
	if err := restoreBackup(bak); err != nil {
		return err
	}
	_ = os.Remove(path)
	flushDNS()
	return nil
}

func saveBackup() error {
	adapters, err := readAdapters()
	if err != nil {
		return err
	}
	if len(adapters) == 0 {
		return fmt.Errorf("не найдены активные сетевые адаптеры")
	}
	if err := os.MkdirAll(filepath.Dir(BackupPath()), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(backupFile{Adapters: adapters}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(BackupPath(), data, 0o644)
}
