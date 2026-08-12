package hosts

import (
	"path/filepath"

	"zapret-manager/internal/zapret"
)

func ConfigPath() string {
	return zapret.HostsConfigPath()
}

func ProxiesConfigPath() string {
	return filepath.Join(zapret.DataRoot(), "proxies.json")
}

func dirOf(path string) string {
	return filepath.Dir(path)
}
