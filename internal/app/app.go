package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"zapret-manager/internal/admin"
	"zapret-manager/internal/dns"
	"zapret-manager/internal/github"
	"zapret-manager/internal/hosts"
	"zapret-manager/internal/selfupdate"
	"zapret-manager/internal/zapret"
)

const channelLatest = "latest"

type Config struct {
	LastStrategy     string          `json:"lastStrategy"`
	LastGameStrategy string          `json:"lastGameStrategy"`
	Channel          string          `json:"channel"`
	TelegramWebBoost *bool           `json:"telegramWebBoost"`
	GameBoost        *bool           `json:"gameBoost"`
	ServiceBoosts    map[string]bool `json:"serviceBoosts"`
	GeoProxy         string          `json:"geoProxy"`
	DNSProfile       string          `json:"dnsProfile"`
}

type ReleaseInfo struct {
	Version     string `json:"version"`
	PublishedAt string `json:"publishedAt"`
}

type ServiceBoostInfo struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Enabled bool   `json:"enabled"`
}

type GeoProxyInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	IP      string `json:"ip"`
	Default bool   `json:"default"`
}

type State struct {
	Admin            bool                `json:"admin"`
	InstallDir       string              `json:"installDir"`
	Installed        bool                `json:"installed"`
	LocalVersion     string              `json:"localVersion"`
	LatestVersion    string              `json:"latestVersion"`
	TargetVersion    string              `json:"targetVersion"`
	Channel          string              `json:"channel"`
	FollowLatest     bool                `json:"followLatest"`
	VersionLabel     string              `json:"versionLabel"`
	Releases         []ReleaseInfo       `json:"releases"`
	Service          zapret.ServiceState    `json:"service"`
	Strategies       []zapret.Strategy      `json:"strategies"`
	GameStrategies   []zapret.GameStrategy  `json:"gameStrategies"`
	Selected         string                 `json:"selected"`
	SelectedGame     string                 `json:"selectedGame"`
	GameFilter       zapret.GameFilter      `json:"gameFilter"`
	TelegramWebBoost bool                   `json:"telegramWebBoost"`
	GameBoost        bool                   `json:"gameBoost"`
	ServiceBoosts    []ServiceBoostInfo     `json:"serviceBoosts"`
	GeoProxy         GeoProxyInfo           `json:"geoProxy"`
	DNSProfile       string                 `json:"dnsProfile"`
	DNSProfiles      []dns.Profile          `json:"dnsProfiles"`
	Busy             bool                   `json:"busy"`
	Progress         float64                `json:"progress"`
	Message          string                 `json:"message"`
	Error            string                 `json:"error"`
	AppUpdateReady   bool                   `json:"appUpdateReady"`
}

type App struct {
	mu       sync.Mutex
	cfg      Config
	gh       *github.Client
	log      *fileLogger
	latest   github.Release
	releases []github.Release
	busy     bool
	progress float64
	message  string
	err      string

	stateCache   State
	stateCacheAt time.Time

	stratMu       sync.Mutex
	stratsCache   []zapret.Strategy
	stratsCacheAt time.Time
	stratsDir     string

	// Once per process: clear foreign zapret/WinDivert and ensure TCP timestamps.
	startPreflightDone bool

	appUpdateReady bool
	appUpdateNew   string
}

const (
	stateCacheTTL  = 2 * time.Second
	stratsCacheTTL = 8 * time.Second
)

func New() *App {
	selfupdate.Cleanup()
	a := &App{gh: github.New(), cfg: loadConfig(), log: newFileLogger()}
	cache := loadGHCache()
	a.applyCache(cache)
	return a
}

func (a *App) GetState() State {
	a.mu.Lock()
	if time.Since(a.stateCacheAt) < stateCacheTTL {
		st := overlayLive(a.stateCache, a)
		a.mu.Unlock()
		return st
	}
	a.mu.Unlock()

	st := a.freshState()

	a.mu.Lock()
	a.stateCache = st
	a.stateCacheAt = time.Now()
	st = overlayLive(st, a)
	a.mu.Unlock()
	return st
}

func overlayLive(st State, a *App) State {
	st.Busy = a.busy
	st.Progress = a.progress
	st.Message = a.message
	st.Error = a.err
	if a.cfg.LastStrategy != "" {
		st.Selected = a.cfg.LastStrategy
	}
	if a.cfg.LastGameStrategy != "" {
		st.SelectedGame = zapret.ResolveGameStrategy(a.cfg.LastGameStrategy).ID
	}
	st.AppUpdateReady = a.appUpdateReady
	return st
}

func (a *App) invalidateStateCache() {
	a.stateCacheAt = time.Time{}
}

func (a *App) freshState() State {
	dir := zapret.DefaultInstallDir()
	svc := zapret.QueryService()
	var strats []zapret.Strategy
	if zapret.CachedInstall(dir).Installed {
		strats = a.listStrategiesCached(dir)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.snapshotFrom(svc, strats)
}

func (a *App) listStrategiesCached(dir string) []zapret.Strategy {
	a.stratMu.Lock()
	defer a.stratMu.Unlock()
	if dir == a.stratsDir && time.Since(a.stratsCacheAt) < stratsCacheTTL && a.stratsCache != nil {
		return a.stratsCache
	}
	strats, err := zapret.ListStrategies(dir)
	if err != nil {
		return nil
	}
	a.stratsCache = strats
	a.stratsCacheAt = time.Now()
	a.stratsDir = dir
	return strats
}

func (a *App) invalidateStrategiesCache() {
	a.stratMu.Lock()
	a.stratsCacheAt = time.Time{}
	a.stratsCache = nil
	a.stratMu.Unlock()
}

func (a *App) goWork(msg string, fn func(context.Context)) State {
	if !a.begin(msg) {
		return a.GetState()
	}
	go func() {
		defer a.end()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		fn(ctx)
	}()
	return a.GetState()
}

func (a *App) Boot() (State, error) {
	// Soft background work: GitHub check must not lock the UI (busy=false).
	a.mu.Lock()
	a.err = ""
	a.message = "Запрос версии с GitHub…"
	a.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		a.bootWork(ctx)
	}()
	go a.selfUpdateWork()
	return a.GetState(), nil
}

func (a *App) selfUpdateWork() {
	time.Sleep(8 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	path, err := selfupdate.Prepare(ctx, func() bool {
		a.mu.Lock()
		defer a.mu.Unlock()
		return a.busy
	})
	if err != nil || path == "" {
		return
	}
	a.mu.Lock()
	a.appUpdateReady = true
	a.appUpdateNew = path
	a.mu.Unlock()
}

func (a *App) ApplyAppUpdate() error {
	a.mu.Lock()
	path := a.appUpdateNew
	ready := a.appUpdateReady
	a.mu.Unlock()
	if !ready || path == "" {
		return nil
	}
	return selfupdate.Apply(path)
}

func (a *App) bootWork(ctx context.Context) {
	a.syncHostsBoosts()
	prev := zapret.QueryService()
	a.refreshLatestQuiet(ctx, prev)
	a.mu.Lock()
	target := TargetVersion(a.cfg.Channel, a.latest.Version())
	a.mu.Unlock()
	if target == "" {
		a.mu.Lock()
		a.err = ""
		a.message = "Локальная сборка, GitHub не запрашивался"
		a.stateCacheAt = time.Time{}
		a.mu.Unlock()
		return
	}

	dir := zapret.DefaultInstallDir()
	needsInstall := !zapret.IsInstalled(dir) || zapret.LocalVersion(dir) != target
	if needsInstall {
		// Download/install must lock controls; skip if the user already started other work.
		if !a.begin("Загрузка " + target + "…") {
			return
		}
		defer a.end()
	}
	_, _ = a.ensureVersion(ctx, target, prev, false)
}

func (a *App) refreshLatestQuiet(ctx context.Context, prev zapret.ServiceState) {
	cache := loadGHCache()
	a.mu.Lock()
	a.applyCache(cache)
	follow := FollowLatest(a.cfg.Channel)
	a.mu.Unlock()
	if !follow {
		return
	}
	if cache.Latest != "" && time.Since(cache.CheckedAt) < versionCacheTTL {
		a.mu.Lock()
		a.err = ""
		a.message = versionReadyMessage(channelLatest, cache.Latest)
		a.stateCacheAt = time.Time{}
		a.mu.Unlock()
		return
	}

	ver, err := a.fetchLatestVersion(ctx)
	if err != nil && cache.Latest != "" {
		a.recordError("GitHub недоступен, кэш " + cache.Latest + ": " + err.Error())
		a.mu.Lock()
		a.err = ""
		a.message = "GitHub недоступен, кэш " + cache.Latest
		a.stateCacheAt = time.Time{}
		a.mu.Unlock()
		return
	}
	if err != nil {
		a.setProgress(0.02, "Повтор без Zapret…")
		_ = zapret.RemoveService(true)
		ver, err = a.fetchLatestVersion(ctx)
		if err != nil {
			a.mu.Lock()
			a.err = ""
			if zapret.IsInstalled(zapret.DefaultInstallDir()) {
				a.message = "Нет связи с GitHub, используется локальная сборка"
			} else {
				a.err = "Не удалось проверить GitHub: " + err.Error()
				a.message = a.err
			}
			errMsg := a.err
			fallback := a.message
			a.stateCacheAt = time.Time{}
			a.mu.Unlock()
			if errMsg != "" {
				a.recordError(errMsg)
			} else {
				a.recordError(fallback + ": " + err.Error())
			}
			a.restoreService(prev)
			return
		}
		a.restoreService(prev)
	}
	cache.Latest = ver
	cache.CheckedAt = time.Now()
	saveGHCache(cache)
	a.mu.Lock()
	a.latest = github.Release{TagName: ver}
	a.err = ""
	a.message = "GitHub: " + ver
	a.stateCacheAt = time.Time{}
	a.mu.Unlock()
}

func (a *App) fetchLatestVersion(ctx context.Context) (string, error) {
	meta, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return a.gh.LatestVersion(meta)
}

func (a *App) LoadReleases() (State, error) {
	cache := loadGHCache()
	if len(cache.Releases) > 0 && time.Since(cache.ReleasesAt) < releasesCacheTTL {
		a.mu.Lock()
		a.applyCache(cache)
		st := a.snapshot()
		a.mu.Unlock()
		return st, nil
	}
	return a.goWork("Список релизов…", func(ctx context.Context) {
		meta, cancel := context.WithTimeout(ctx, 12*time.Second)
		defer cancel()
		list, err := a.gh.ListReleases(meta)
		if err != nil {
			if len(cache.Releases) > 0 {
				a.recordError("Не удалось получить релизы, кэш: " + err.Error())
				a.mu.Lock()
				a.applyCache(cache)
				a.err = ""
				a.message = "Список релизов из кэша"
				a.mu.Unlock()
				return
			}
			a.fail("Не удалось получить релизы: " + err.Error())
			return
		}
		infos := make([]ReleaseInfo, 0, len(list))
		for _, rel := range list {
			info := ReleaseInfo{Version: rel.Version()}
			if !rel.PublishedAt.IsZero() {
				info.PublishedAt = rel.PublishedAt.Local().Format("02.01.2006")
			}
			infos = append(infos, info)
		}
		cache := loadGHCache()
		cache.Releases = infos
		cache.ReleasesAt = time.Now()
		if cache.Latest == "" && len(infos) > 0 {
			cache.Latest = infos[0].Version
			cache.CheckedAt = time.Now()
		}
		saveGHCache(cache)
		a.mu.Lock()
		a.releases = list
		if a.latest.TagName == "" && len(list) > 0 {
			a.latest = list[0]
		}
		a.err = ""
		a.message = ""
		a.mu.Unlock()
	}), nil
}

func (a *App) SelectVersion(channel string) (State, error) {
	if channel == "" {
		channel = channelLatest
	}
	a.mu.Lock()
	a.cfg.Channel = channel
	_ = saveConfig(a.cfg)
	a.mu.Unlock()
	return a.goWork("Смена версии…", func(ctx context.Context) {
		prev := zapret.QueryService()
		if FollowLatest(channel) {
			a.refreshLatestQuiet(ctx, prev)
		}
		a.mu.Lock()
		target := TargetVersion(a.cfg.Channel, a.latest.Version())
		a.mu.Unlock()
		if target == "" {
			a.fail("Не удалось определить версию")
			return
		}
		_, _ = a.ensureVersion(ctx, target, prev, false)
	}), nil
}

func (a *App) SelectStrategy(name string) (State, error) {
	a.mu.Lock()
	a.cfg.LastStrategy = name
	_ = saveConfig(a.cfg)
	a.invalidateStateCache()
	a.mu.Unlock()

	prev := zapret.QueryService()
	if !serviceActive(prev) {
		return a.GetState(), nil
	}
	return a.goWork("Смена стратегии…", func(context.Context) {
		dir := zapret.DefaultInstallDir()
		if err := a.enableZapret(dir, name); err != nil {
			a.fail(err.Error())
			return
		}
		a.mu.Lock()
		a.err = ""
		a.message = "Основная стратегия: " + name
		a.mu.Unlock()
	}), nil
}

func (a *App) SelectGameStrategy(id string) (State, error) {
	gs := zapret.ResolveGameStrategy(id)
	a.mu.Lock()
	a.cfg.LastGameStrategy = gs.ID
	_ = saveConfig(a.cfg)
	a.invalidateStateCache()
	boost := a.cfg.IsGameBoost()
	mainName := a.cfg.LastStrategy
	a.mu.Unlock()

	prev := zapret.QueryService()
	if !boost || !serviceActive(prev) {
		a.mu.Lock()
		a.err = ""
		a.message = "Игровая стратегия: " + gs.Name
		st := a.snapshot()
		a.mu.Unlock()
		return st, nil
	}
	return a.goWork("Смена игровой стратегии…", func(context.Context) {
		dir := zapret.DefaultInstallDir()
		if mainName == "" {
			mainName = a.strategyForRestart(dir, prev.Strategy)
		}
		if err := a.enableZapret(dir, mainName); err != nil {
			a.fail(err.Error())
			return
		}
		a.mu.Lock()
		a.err = ""
		a.message = "Игровая стратегия: " + gs.Name
		a.mu.Unlock()
	}), nil
}

func (a *App) SetGameBoost(enabled bool) (State, error) {
	a.mu.Lock()
	a.cfg.GameBoost = boolPtr(enabled)
	_ = saveConfig(a.cfg)
	mainName := a.cfg.LastStrategy
	a.mu.Unlock()

	prev := zapret.QueryService()
	if !serviceActive(prev) {
		a.mu.Lock()
		a.err = ""
		if enabled {
			a.message = "Ускорение игр включено"
		} else {
			a.message = "Ускорение игр выключено"
		}
		st := a.snapshot()
		a.mu.Unlock()
		return st, nil
	}
	msg := "Ускорение игр выключено"
	if enabled {
		msg = "Ускорение игр включено"
	}
	return a.goWork(msg+"…", func(context.Context) {
		dir := zapret.DefaultInstallDir()
		if mainName == "" {
			mainName = a.strategyForRestart(dir, prev.Strategy)
		}
		if err := a.enableZapret(dir, mainName); err != nil {
			a.fail(err.Error())
			return
		}
		a.mu.Lock()
		a.err = ""
		a.message = msg
		a.mu.Unlock()
	}), nil
}

func (a *App) enableZapret(dir, strategyName string) error {
	a.mu.Lock()
	if strategyName == "" {
		strategyName = a.cfg.LastStrategy
	}
	var game *zapret.GameStrategy
	if a.cfg.IsGameBoost() {
		gs := zapret.ResolveGameStrategy(a.cfg.LastGameStrategy)
		game = &gs
	}
	a.mu.Unlock()
	return zapret.EnableService(dir, strategyName, game)
}

// runStartPreflight clears foreign Flowseal/other zapret + WinDivert and
// enables TCP timestamps. Runs once per app process, on first Start.
func (a *App) runStartPreflight() {
	a.mu.Lock()
	if a.startPreflightDone {
		a.mu.Unlock()
		return
	}
	a.startPreflightDone = true
	a.mu.Unlock()

	a.setProgress(0.05, "Проверка чужих zapret/WinDivert…")
	type cleanupRes struct {
		removed []string
		err     error
	}
	done := make(chan cleanupRes, 1)
	go func() {
		removed, err := zapret.CleanupConflicts()
		done <- cleanupRes{removed, err}
	}()
	if err := zapret.EnsureTCPTimestamps(); err != nil {
		a.recordError("TCP timestamps: " + err.Error())
	}
	res := <-done
	if res.err != nil {
		a.recordError("очистка конфликтов: " + res.err.Error())
	} else if len(res.removed) > 0 {
		a.mu.Lock()
		a.message = "Убраны конфликты: " + strings.Join(res.removed, ", ")
		a.mu.Unlock()
	}
}

func (a *App) SetGameFilter(mode string) (State, error) {
	dir := zapret.DefaultInstallDir()
	if err := zapret.SaveGameFilter(dir, mode); err != nil {
		return a.fail(err.Error())
	}
	label := zapret.LoadGameFilter(dir).Label()
	prev := zapret.QueryService()
	if !serviceActive(prev) {
		a.mu.Lock()
		a.err = ""
		a.message = "Game Filter: " + label
		st := a.snapshot()
		a.mu.Unlock()
		return st, nil
	}
	a.mu.Lock()
	name := a.cfg.LastStrategy
	a.mu.Unlock()
	return a.goWork("Применение Game Filter…", func(context.Context) {
		if name == "" {
			name = a.strategyForRestart(dir, prev.Strategy)
		}
		if name == "" {
			a.mu.Lock()
			a.err = ""
			a.message = "Game Filter: " + label
			a.mu.Unlock()
			return
		}
		if err := a.enableZapret(dir, name); err != nil {
			a.fail("Game Filter сохранён, но службу не удалось перезапустить: " + err.Error())
			return
		}
		a.mu.Lock()
		a.err = ""
		a.message = "Game Filter: " + label
		a.mu.Unlock()
	}), nil
}

func serviceActive(st zapret.ServiceState) bool {
	return st.Status == "running" || st.Status == "starting" || st.Winws
}

func (a *App) SetTelegramWebBoost(enabled bool) (State, error) {
	_ = hosts.Reload()
	a.mu.Lock()
	a.cfg.TelegramWebBoost = boolPtr(enabled)
	_ = saveConfig(a.cfg)
	a.mu.Unlock()
	if err := a.syncHostsToServiceState(); err != nil {
		return a.fail("Telegram Web: " + err.Error())
	}
	a.mu.Lock()
	a.err = ""
	if enabled {
		a.message = "Ускорение Telegram Web включено"
	} else {
		a.message = "Ускорение Telegram Web выключено"
	}
	st := a.snapshot()
	a.mu.Unlock()
	return st, nil
}

func (a *App) SetServiceBoost(id string, enabled bool) (State, error) {
	_ = hosts.Reload()
	p, ok := hosts.ProfileByID(id)
	if !ok || id == hosts.IDTelegram {
		return a.fail("Неизвестный сервис")
	}
	a.mu.Lock()
	if a.cfg.ServiceBoosts == nil {
		a.cfg.ServiceBoosts = map[string]bool{}
	}
	a.cfg.ServiceBoosts[id] = enabled
	_ = saveConfig(a.cfg)
	a.mu.Unlock()
	if err := a.syncHostsToServiceState(); err != nil {
		return a.fail(p.Title + ": " + err.Error())
	}
	a.mu.Lock()
	a.err = ""
	if enabled {
		a.message = "Ускорение «" + p.Title + "» включено"
	} else {
		a.message = "Ускорение «" + p.Title + "» выключено"
	}
	st := a.snapshot()
	a.mu.Unlock()
	return st, nil
}

func (a *App) SetGeoProxy(ip string) (State, error) {
	_ = hosts.Reload()
	_ = hosts.ReloadProxies()
	ip = hosts.NormalizeProxyIP(ip)
	a.mu.Lock()
	a.cfg.GeoProxy = ip
	_ = saveConfig(a.cfg)
	a.mu.Unlock()
	if err := a.syncHostsToServiceState(); err != nil {
		return a.fail("GeoHide Proxy: " + err.Error())
	}
	name := ip
	if p, ok := hosts.ProxyByIP(ip); ok {
		name = p.Name
	}
	a.mu.Lock()
	a.err = ""
	a.message = "GeoHide Proxy: " + name + " (" + ip + ")"
	st := a.snapshot()
	a.mu.Unlock()
	return st, nil
}

func (a *App) PingGeoProxies() []hosts.ProxyStatus {
	return hosts.PingProxies()
}

func (a *App) syncHostsToServiceState() error {
	_ = hosts.Reload()
	dir := zapret.DefaultInstallDir()
	prev := zapret.QueryService()
	if !serviceActive(prev) {
		_ = zapret.ClearBoostHostlist(dir)
		return hosts.ClearAll()
	}
	a.mu.Lock()
	enabled := a.cfg.hostsEnabledMap()
	proxy := a.cfg.GeoProxyIP()
	strategy := a.cfg.LastStrategy
	a.mu.Unlock()
	if err := hosts.ApplyAll(enabled, proxy); err != nil {
		return err
	}
	changed, err := zapret.SyncBoostHostlist(dir, hosts.EnabledDomains(enabled))
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	// winws reads hostlists at process start; restart so new domains get dpi-desync.
	if strategy == "" {
		strategy = a.strategyForRestart(dir, prev.Strategy)
	}
	if strategy == "" {
		return nil
	}
	return a.enableZapret(dir, strategy)
}

func (a *App) syncHostsBoosts() {
	if err := hosts.Reload(); err != nil {
		a.recordError("hosts.json: " + err.Error())
	}
	_ = hosts.ReloadProxies()
	a.mu.Lock()
	a.cfg = normalizeConfig(a.cfg)
	_ = saveConfig(a.cfg)
	a.mu.Unlock()
	if err := a.syncHostsToServiceState(); err != nil {
		a.recordError("hosts: " + err.Error())
		a.mu.Lock()
		a.err = "Не удалось обновить hosts: " + err.Error()
		a.message = a.err
		a.mu.Unlock()
	}
}

func (a *App) Start() (State, error) {
	return a.goWork("Запуск службы…", func(context.Context) {
		dir := zapret.DefaultInstallDir()
		a.mu.Lock()
		name := a.cfg.LastStrategy
		a.mu.Unlock()
		if name == "" {
			strats, _ := zapret.ListStrategies(dir)
			name = zapret.PickDefault(strats)
			if name == "" {
				a.fail("Сначала дождитесь загрузки Zapret")
				return
			}
			a.mu.Lock()
			a.cfg.LastStrategy = name
			_ = saveConfig(a.cfg)
			a.mu.Unlock()
		}
		a.runStartPreflight()
		if err := a.enableZapret(dir, name); err != nil {
			a.fail(err.Error())
			return
		}
		if err := a.syncHostsToServiceState(); err != nil {
			a.recordError("hosts: " + err.Error())
		}
		if err := a.syncDNSToServiceState(true); err != nil {
			a.recordError("DNS: " + err.Error())
		}
		a.mu.Lock()
		a.err = ""
		a.message = "Служба запущена: " + name
		a.mu.Unlock()
	}), nil
}

func (a *App) Stop() (State, error) {
	return a.goWork("Остановка службы…", func(context.Context) {
		if err := zapret.StopService(); err != nil {
			a.fail(err.Error())
			return
		}
		_ = zapret.ClearBoostHostlist(zapret.DefaultInstallDir())
		if err := hosts.ClearAll(); err != nil {
			a.recordError("hosts: " + err.Error())
		}
		if err := a.syncDNSToServiceState(false); err != nil {
			a.recordError("DNS: " + err.Error())
		}
		a.mu.Lock()
		a.message = "Служба остановлена"
		a.err = ""
		a.mu.Unlock()
	}), nil
}

func (a *App) Remove() (State, error) {
	return a.goWork("Удаление службы…", func(context.Context) {
		if err := zapret.RemoveService(true); err != nil {
			a.fail(err.Error())
			return
		}
		_ = zapret.ClearBoostHostlist(zapret.DefaultInstallDir())
		if err := hosts.ClearAll(); err != nil {
			a.recordError("hosts: " + err.Error())
		}
		if err := a.syncDNSToServiceState(false); err != nil {
			a.recordError("DNS: " + err.Error())
		}
		a.mu.Lock()
		a.message = "Служба и WinDivert удалены"
		a.err = ""
		a.mu.Unlock()
	}), nil
}

func (a *App) SetDNSProfile(id string) (State, error) {
	id = dns.NormalizeID(id)
	title := id
	if p, ok := dns.ProfileByID(id); ok {
		title = p.Title
	}
	a.mu.Lock()
	a.cfg.DNSProfile = id
	_ = saveConfig(a.cfg)
	a.err = ""
	a.message = "DNS: " + title
	st := a.snapshot()
	a.mu.Unlock()
	return st, nil
}

func (a *App) syncDNSToServiceState(active bool) error {
	a.mu.Lock()
	profile := a.cfg.DNSProfile
	a.mu.Unlock()
	return dns.SyncForService(active, profile)
}

func (a *App) ensureVersion(ctx context.Context, target string, prev zapret.ServiceState, paused bool) (State, error) {
	dir := zapret.DefaultInstallDir()
	local := zapret.LocalVersion(dir)
	installed := zapret.IsInstalled(dir)
	if installed && local == target {
		a.restoreService(prev)
		a.mu.Lock()
		a.err = ""
		a.message = versionReadyMessage(a.cfg.Channel, target)
		a.stateCacheAt = time.Time{}
		st := a.snapshot()
		a.mu.Unlock()
		return st, nil
	}

	rel, ok := a.releaseByVersion(target)
	if !ok {
		rel = github.Release{TagName: target}
	}

	wasInstalled := prev.Installed || paused
	wasRunning := prev.Status == "running" || prev.Winws
	if !paused {
		st := zapret.QueryService()
		wasInstalled = st.Installed || wasInstalled
		wasRunning = wasRunning || st.Status == "running" || st.Winws
	}

	if err := a.downloadAndInstall(ctx, rel.Version(), dir); err != nil {
		a.restoreService(prev)
		return a.fail(err.Error())
	}

	if wasInstalled || wasRunning {
		a.setProgress(0.97, "Пересоздание службы…")
		name := a.strategyForRestart(dir, prev.Strategy)
		if name != "" {
			if err := a.enableZapret(dir, name); err != nil {
				return a.fail("Версия установлена, но службу не удалось запустить: " + err.Error())
			}
		}
	}
	if err := a.syncHostsToServiceState(); err != nil {
		a.recordError("hosts: " + err.Error())
	}

	a.mu.Lock()
	a.progress = 1
	a.err = ""
	a.message = "Установлено " + target
	st := a.snapshot()
	a.mu.Unlock()
	return st, nil
}

func (a *App) restoreService(prev zapret.ServiceState) {
	if prev.Status != "running" && !prev.Winws {
		return
	}
	if zapret.QueryService().Status == "running" {
		return
	}
	dir := zapret.DefaultInstallDir()
	name := a.strategyForRestart(dir, prev.Strategy)
	if name == "" {
		return
	}
	_ = a.enableZapret(dir, name)
}

func (a *App) downloadAndInstall(ctx context.Context, version, dir string) error {
	a.setProgress(0.03, "Остановка Zapret перед загрузкой…")
	_ = zapret.RemoveService(true)

	cache := zapret.CacheDir()
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return err
	}
	zipPath := filepath.Join(cache, github.ZipName(version))
	url := github.ZipDownloadURL(version)
	a.setProgress(0.05, "Загрузка "+version+"…")
	err := a.gh.Download(ctx, url, zipPath, func(got, total int64) {
		a.mu.Lock()
		if total > 0 {
			a.progress = 0.05 + float64(got)/float64(total)*0.8
		} else if got > 0 {
			a.progress = 0.15
		}
		a.message = fmt.Sprintf("Загрузка %s… %s", version, formatBytes(got))
		a.mu.Unlock()
	})
	if err != nil {
		return fmt.Errorf("загрузка: %w", err)
	}
	a.setProgress(0.93, "Распаковка…")
	if err := zapret.InstallZip(zipPath, dir); err != nil {
		return fmt.Errorf("установка: %w", err)
	}
	a.invalidateStrategiesCache()
	return nil
}

func (a *App) releaseByVersion(version string) (github.Release, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if rel, ok := github.FindRelease(a.releases, version); ok {
		return rel, true
	}
	if a.latest.Version() == version {
		return a.latest, true
	}
	return github.Release{}, false
}

func (a *App) strategyForRestart(dir, previous string) string {
	a.mu.Lock()
	name := a.cfg.LastStrategy
	a.mu.Unlock()
	if name == "" {
		name = previous
	}
	strats, _ := zapret.ListStrategies(dir)
	for _, s := range strats {
		if s.Name == name {
			return name
		}
	}
	if len(strats) > 0 {
		return zapret.PickDefault(strats)
	}
	return ""
}

// snapshot must be called with a.mu held.
func (a *App) snapshot() State {
	dir := zapret.DefaultInstallDir()
	svc := zapret.QueryService()
	var strats []zapret.Strategy
	if zapret.CachedInstall(dir).Installed {
		strats = a.listStrategiesCached(dir)
	}
	st := a.snapshotFrom(svc, strats)
	a.stateCache = st
	a.stateCacheAt = time.Now()
	return st
}

// snapshotFrom must be called with a.mu held.
func (a *App) snapshotFrom(svc zapret.ServiceState, strats []zapret.Strategy) State {
	dir := zapret.DefaultInstallDir()
	info := zapret.CachedInstall(dir)
	channel := NormalizeChannel(a.cfg.Channel)
	latest := a.latest.Version()
	target := TargetVersion(channel, latest)
	st := State{
		Admin:            admin.IsElevated(),
		InstallDir:       dir,
		Installed:        info.Installed,
		LocalVersion:     info.Version,
		LatestVersion:    latest,
		TargetVersion:    target,
		Channel:          channel,
		FollowLatest:     FollowLatest(channel),
		Busy:             a.busy,
		Progress:         a.progress,
		Message:          a.message,
		Error:            a.err,
		Service:          svc,
		Strategies:       strats,
		GameStrategies:   zapret.GameStrategies(),
		GameFilter:       info.Filter,
		TelegramWebBoost: a.cfg.IsTelegramWebBoost(),
		GameBoost:        a.cfg.IsGameBoost(),
		ServiceBoosts:    a.cfg.serviceBoostInfos(),
		GeoProxy:         a.cfg.geoProxyInfo(),
		DNSProfile:       dns.NormalizeID(a.cfg.DNSProfile),
		DNSProfiles:      dns.Profiles(),
		Selected:         a.cfg.LastStrategy,
		SelectedGame:     zapret.ResolveGameStrategy(a.cfg.LastGameStrategy).ID,
	}
	if FollowLatest(channel) {
		st.VersionLabel = "Актуальная"
	} else {
		st.VersionLabel = target
	}
	for _, rel := range a.releases {
		info := ReleaseInfo{Version: rel.Version()}
		if !rel.PublishedAt.IsZero() {
			info.PublishedAt = rel.PublishedAt.Local().Format("02.01.2006")
		}
		st.Releases = append(st.Releases, info)
	}
	if st.Selected == "" && st.Service.Strategy != "" {
		st.Selected = st.Service.Strategy
	}
	if st.Selected == "" && len(st.Strategies) > 0 {
		st.Selected = zapret.PickDefault(st.Strategies)
	}
	return st
}

func (a *App) begin(msg string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.busy {
		return false
	}
	a.busy = true
	a.progress = 0
	a.err = ""
	a.message = msg
	a.stateCacheAt = time.Time{}
	return true
}

func (a *App) end() {
	a.mu.Lock()
	a.busy = false
	a.stateCacheAt = time.Time{}
	a.mu.Unlock()
}

func (a *App) setProgress(p float64, msg string) {
	a.mu.Lock()
	a.progress = p
	a.message = msg
	a.mu.Unlock()
}

func (a *App) GetLogs() LogsView {
	if a.log == nil {
		return LogsView{Path: zapret.LogFile(), Entries: []LogEntry{}}
	}
	return a.log.View()
}

func (a *App) LogError(msg string) {
	a.recordError(msg)
}

func (a *App) recordError(msg string) {
	if a.log != nil {
		a.log.Error(msg)
	}
}

func (a *App) fail(msg string) (State, error) {
	a.recordError(msg)
	a.mu.Lock()
	a.err = msg
	a.message = msg
	st := a.snapshot()
	a.mu.Unlock()
	return st, fmt.Errorf("%s", msg)
}

func FollowLatest(channel string) bool {
	return channel == "" || channel == channelLatest
}

func NormalizeChannel(channel string) string {
	if FollowLatest(channel) {
		return channelLatest
	}
	return channel
}

func TargetVersion(channel, latest string) string {
	if FollowLatest(channel) {
		return latest
	}
	return channel
}

func versionReadyMessage(channel, version string) string {
	if FollowLatest(channel) {
		return "Актуальная версия " + version
	}
	return "Закреплена версия " + version
}

func configPath() (string, error) {
	dir := zapret.DataRoot()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func loadConfig() Config {
	cfg := normalizeConfig(Config{Channel: channelLatest})
	path, err := configPath()
	if err != nil {
		return cfg
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	parsed := Config{Channel: channelLatest}
	_ = json.Unmarshal(data, &parsed)
	return normalizeConfig(parsed)
}

func saveConfig(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	cfg = normalizeConfig(cfg)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func normalizeConfig(cfg Config) Config {
	cfg.Channel = NormalizeChannel(cfg.Channel)
	if cfg.TelegramWebBoost == nil {
		cfg.TelegramWebBoost = boolPtr(true)
	}
	if cfg.GameBoost == nil {
		cfg.GameBoost = boolPtr(true)
	}
	cfg.LastGameStrategy = zapret.ResolveGameStrategy(cfg.LastGameStrategy).ID
	cfg.DNSProfile = dns.NormalizeID(cfg.DNSProfile)
	if cfg.ServiceBoosts == nil {
		cfg.ServiceBoosts = map[string]bool{}
	}
	for _, p := range hosts.ExtraProfiles() {
		if _, ok := cfg.ServiceBoosts[p.ID]; !ok {
			cfg.ServiceBoosts[p.ID] = p.DefaultOn
		}
	}
	cfg.GeoProxy = hosts.NormalizeProxyIP(cfg.GeoProxy)
	return cfg
}

func (c Config) IsTelegramWebBoost() bool {
	if c.TelegramWebBoost == nil {
		return true
	}
	return *c.TelegramWebBoost
}

func (c Config) IsGameBoost() bool {
	if c.GameBoost == nil {
		return true
	}
	return *c.GameBoost
}

func (c Config) IsServiceBoost(id string) bool {
	if c.ServiceBoosts != nil {
		if v, ok := c.ServiceBoosts[id]; ok {
			return v
		}
	}
	return hosts.DefaultOn(id)
}

func (c Config) GeoProxyIP() string {
	return hosts.NormalizeProxyIP(c.GeoProxy)
}

func (c Config) geoProxyInfo() GeoProxyInfo {
	ip := c.GeoProxyIP()
	if p, ok := hosts.ProxyByIP(ip); ok {
		return GeoProxyInfo{ID: p.ID, Name: p.Name, IP: p.IP, Default: p.Default}
	}
	return GeoProxyInfo{ID: ip, Name: "Custom", IP: ip}
}

func (c Config) serviceBoostInfos() []ServiceBoostInfo {
	profiles := hosts.ExtraProfiles()
	out := make([]ServiceBoostInfo, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, ServiceBoostInfo{
			ID:      p.ID,
			Title:   p.Title,
			Enabled: c.IsServiceBoost(p.ID),
		})
	}
	return out
}

func (c Config) hostsEnabledMap() map[string]bool {
	out := map[string]bool{hosts.IDTelegram: c.IsTelegramWebBoost()}
	for _, p := range hosts.ExtraProfiles() {
		out[p.ID] = c.IsServiceBoost(p.ID)
	}
	return out
}

func boolPtr(v bool) *bool {
	return &v
}

func formatBytes(n int64) string {
	const kb = 1024
	switch {
	case n >= kb*kb:
		return fmt.Sprintf("%.1f МБ", float64(n)/float64(kb*kb))
	case n >= kb:
		return fmt.Sprintf("%.0f КБ", float64(n)/kb)
	default:
		return fmt.Sprintf("%d Б", n)
	}
}
