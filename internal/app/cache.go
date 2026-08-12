package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"zapret-manager/internal/github"
	"zapret-manager/internal/zapret"
)

const (
	versionCacheTTL  = 30 * time.Minute
	releasesCacheTTL = 6 * time.Hour
)

type ghCache struct {
	Latest     string        `json:"latest"`
	CheckedAt  time.Time     `json:"checkedAt"`
	Releases   []ReleaseInfo `json:"releases"`
	ReleasesAt time.Time     `json:"releasesAt"`
}

func cacheFile() string {
	return filepath.Join(zapret.DataRoot(), "github-cache.json")
}

func loadGHCache() ghCache {
	var c ghCache
	data, err := os.ReadFile(cacheFile())
	if err != nil {
		return c
	}
	_ = json.Unmarshal(data, &c)
	return c
}

func saveGHCache(c ghCache) {
	_ = os.MkdirAll(zapret.DataRoot(), 0o755)
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(cacheFile(), data, 0o644)
}

func (a *App) applyCache(c ghCache) {
	if c.Latest != "" {
		a.latest = github.Release{TagName: c.Latest}
	}
	if len(c.Releases) > 0 {
		rels := make([]github.Release, 0, len(c.Releases))
		for _, info := range c.Releases {
			rel := github.Release{TagName: info.Version}
			if info.PublishedAt != "" {
				if t, err := time.Parse("02.01.2006", info.PublishedAt); err == nil {
					rel.PublishedAt = t
				}
			}
			rels = append(rels, rel)
		}
		a.releases = rels
	}
}
