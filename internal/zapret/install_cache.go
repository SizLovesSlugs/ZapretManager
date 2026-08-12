package zapret

import (
	"sync"
	"time"
)

type installInfo struct {
	Installed bool
	Version   string
	Filter    GameFilter
}

var (
	instMu   sync.Mutex
	instAt   time.Time
	instRoot string
	instVal  installInfo
)

const installCacheTTL = 2 * time.Second

func CachedInstall(root string) installInfo {
	instMu.Lock()
	defer instMu.Unlock()
	if root == instRoot && time.Since(instAt) < installCacheTTL {
		return instVal
	}
	instVal = installInfo{
		Installed: IsInstalled(root),
		Version:   LocalVersion(root),
		Filter:    LoadGameFilter(root),
	}
	instRoot = root
	instAt = time.Now()
	return instVal
}

func InvalidateInstallCache() {
	instMu.Lock()
	instAt = time.Time{}
	instMu.Unlock()
}
