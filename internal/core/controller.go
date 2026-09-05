package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/he11ah0und/logger"
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core/api"
	"sing-box-ez/internal/core/api/clash"
	"sing-box-ez/internal/core/api/singbox"
	"sing-box-ez/internal/core/inboundstyle"
	"sing-box-ez/internal/framework"
	"sing-box-ez/internal/framework/fs"
	fwnet "sing-box-ez/internal/framework/net"
	"sing-box-ez/internal/framework/updater"
	"sing-box-ez/internal/framework/util/openfile"
	"sing-box-ez/internal/singboxconfig"
)

// Sentinel errors returned by PrepareConfig so callers can react specifically.
var (
	ErrCoreMissing    = errors.New("core not found. Please download it first")
	ErrNoActiveConfig = errors.New("no active config. Please add and activate a config in the Configs tab")
	// ErrStartCancelled marks a start flow aborted by the user (CancelStart).
	ErrStartCancelled = errors.New("start cancelled")
	// ErrPrivilegesRequired marks a start refused because the config needs
	// elevated OS privileges (tun/redirect/tproxy inbound) the app does not
	// have: setcap on the core binary (Linux) or an admin restart (Windows).
	ErrPrivilegesRequired = errors.New("config requires elevated privileges (tun/transparent inbound): apply setcap to the core binary (Linux) or restart as administrator (Windows)")
)

// Controller is the core application API used by both CLI and GUI.
// It owns the sing-box process manager, config lifecycle, and privilege helpers.
type Controller struct {
	cfg        *config.AppConfig
	fwApp      *framework.App
	manager    *Manager
	processor  *CoreLogProcessor
	terminal   *logger.LogTerminal
	privileges *PrivilegeController
	apiInfo    *api.Info
	apiClient  api.CoreAPIClient

	// configUpdateMu serializes config downloads (start flow, background
	// auto-update, manual update) so a second updater re-checks staleness
	// after the first one finishes instead of downloading the same profile
	// twice and triggering a redundant restart.
	configUpdateMu sync.Mutex

	// downloadCancel cancels the in-flight config download of the start
	// flow (the GUI's "skip update" button); nil when no download runs.
	downloadMu     sync.Mutex
	downloadCancel context.CancelFunc

	// proxyAddr is the listen address of the running core's mixed inbound,
	// captured from the final config tree at (re)start; "" when the core is
	// stopped or has no mixed inbound.
	proxyMu   sync.Mutex
	proxyAddr string

	// skipNextRefresh makes the next start flow skip the active config
	// refresh exactly once — the "retry via core" flow updates the config
	// through the running core's proxy instead of before the start.
	skipNextRefresh atomic.Bool

	// OnConfigDownloadFailed is invoked when the start flow could not
	// download the active config and fell back to the cached copy (a genuine
	// failure, not a user skip). The GUI offers a retry through the running
	// core's proxy.
	OnConfigDownloadFailed func(name string, err error)
}

// NewController creates a new controller and wires framework services.
func NewController(cfg *config.AppConfig, fwApp *framework.App, parent *logger.LogTerminal) *Controller {
	active := cfg.GetActiveConfig()

	var coreUpdater *updater.Manager
	if fwApp != nil {
		for _, m := range fwApp.Updaters {
			if m.Name == "core-updater" {
				coreUpdater = m
				break
			}
		}
	}

	// Give the core manager its own scoped FS logger so I/O errors are
	// attributed to [core][fs] instead of the generic root terminal.
	coreFS := fs.NewOSWithLog(cfg.DataDir, parent.Allocate("core").Allocate("fs"))
	manager := NewManager(cfg.DataDir, coreFS, coreUpdater, fwApp.Logger)
	manager.SetBinaryResolver(func() string { return ResolveCoreBinary(cfg, cfg.DataDir) })
	if active != nil {
		manager.SetConfigName(active.Name)
	}
	// On windows the app itself must run as administrator; that is what
	// elevates the core. On linux elevation is handled exclusively via
	// setcap (a setcap-applied core binary runs directly).
	if runtime.GOOS == "windows" {
		manager.SetElevated(IsAdmin())
	}

	logWriter := NewCoreLogWriter()
	manager.SetLogOutput(logWriter)

	terminal := parent.Allocate("controller")
	processor := NewCoreLogProcessor(cfg, manager, logWriter, terminal)
	processor.Start()

	privileges := NewPrivilegeController(cfg, manager, terminal)

	c := &Controller{
		cfg:        cfg,
		fwApp:      fwApp,
		manager:    manager,
		processor:  processor,
		terminal:   terminal,
		privileges: privileges,
	}
	c.manager.SetConfigTransform(func(data []byte) ([]byte, error) {
		data, err := c.applyLogOverride(data)
		if err != nil {
			return nil, err
		}
		return c.applyInboundsOverride(data, c.cfg.GetActiveConfig())
	})
	return c
}

// Close shuts down the controller.
func (c *Controller) Close() {
	if c.manager.IsRunning() {
		_ = c.manager.Stop()
	}
	if c.processor != nil {
		c.processor.Stop()
	}
	if c.apiClient != nil {
		_ = c.apiClient.Close()
		c.apiClient = nil
	}
}

// Config returns the application configuration.
func (c *Controller) Config() *config.AppConfig {
	return c.cfg
}

// GetConfigs returns all configured profiles.
func (c *Controller) GetConfigs() []config.ConfigRecord {
	return c.cfg.GetConfigs()
}

// GetActiveConfig returns the currently active profile.
func (c *Controller) GetActiveConfig() *config.ConfigRecord {
	return c.cfg.GetActiveConfig()
}

// GetActiveName returns the name of the active profile.
func (c *Controller) GetActiveName() string {
	return c.cfg.GetActiveName()
}

// Framework returns the framework app.
func (c *Controller) Framework() *framework.App {
	return c.fwApp
}

// Manager returns the core process manager.
func (c *Controller) Manager() *Manager {
	return c.manager
}

// LogProcessor returns the core log processor.
func (c *Controller) LogProcessor() *CoreLogProcessor {
	return c.processor
}

// Terminal returns the controller's logger terminal.
func (c *Controller) Terminal() *logger.LogTerminal {
	return c.terminal
}

// ---------- Logging ----------

func (c *Controller) Log(msg string) {
	c.fwApp.Logger.Log(msg)
}

func (c *Controller) GetLogLines() []string {
	return c.fwApp.Logger.GetLines()
}

func (c *Controller) GetLogLinesAtLeast(minLevel logger.LogLevel) []string {
	return c.fwApp.Logger.GetLinesAtLeast(minLevel)
}

func (c *Controller) ClearLogs() {
	c.fwApp.Logger.Clear()
}

func (c *Controller) GetCoreLogLines() []string {
	return c.processor.LogBuffer().GetLines()
}

func (c *Controller) GetCoreLogCleanLines() []string {
	return c.processor.LogBuffer().GetCleanLines()
}

func (c *Controller) ClearCoreLogs() {
	c.processor.LogBuffer().Clear()
}

// ---------- Core lifecycle ----------

func (c *Controller) PrepareConfig(ctx context.Context) (*config.ConfigRecord, error) {
	active := c.cfg.GetActiveConfig()
	if active == nil {
		return nil, ErrNoActiveConfig
	}

	if !c.CoreExists() {
		return nil, ErrCoreMissing
	}
	if _, err := CheckCoreCompatibility(c.manager.coreBinary()); err != nil {
		return nil, err
	}
	c.manager.SetConfigURL(active.URL)
	c.manager.SetConfigName(active.Name)

	if active.IsLocal() {
		if !c.HasCachedConfig(active.Name) {
			if err := c.manager.CreateLocalConfig(active.Name); err != nil {
				return nil, fmt.Errorf("failed to create local config: %w", err)
			}
		}
		return active, nil
	}

	if err := c.refreshActiveConfig(ctx); err != nil {
		return nil, err
	}

	if active.Hash == "" && c.HasCachedConfig(active.Name) {
		if data, err := c.manager.ReadConfigByName(active.Name); err == nil {
			c.saveConfigHash(active.Name, data)
		}
	}

	return active, nil
}

// refreshActiveConfig downloads the active remote config when it is due. The
// due check runs again under the config update lock, so a concurrent
// background or manual update that already refreshed the profile makes the
// start flow skip its own download.
func (c *Controller) refreshActiveConfig(ctx context.Context) error {
	c.configUpdateMu.Lock()
	defer c.configUpdateMu.Unlock()

	if c.skipNextRefresh.CompareAndSwap(true, false) {
		c.terminal.TInfof("core.controller.config_update_skipped_once")
		return nil
	}

	active := c.cfg.GetActiveConfig()
	if active == nil {
		return ErrNoActiveConfig
	}
	if active.IsLocal() {
		return nil
	}
	if !active.ShouldUpdate() && c.HasCachedConfig(active.Name) &&
		!(c.cfg.MustGet("updates", "auto_update_on_hash_mismatch").Bool() && c.IsConfigHashMismatch(active.Name)) {
		return nil
	}

	c.terminal.TInfof("core.controller.config_updating")
	// The download runs on a child context so the GUI's "skip" button can
	// abort just the download (falling back to the cached config) without
	// cancelling the whole start flow.
	downloadCtx, downloadCancel := context.WithCancel(ctx)
	c.downloadMu.Lock()
	c.downloadCancel = downloadCancel
	c.downloadMu.Unlock()
	data, err := c.manager.UpdateConfig(downloadCtx)
	// A user skip cancels downloadCtx while the parent ctx stays alive; decide
	// before downloadCancel() below makes downloadCtx.Err() always non-nil.
	skipped := err != nil && ctx.Err() == nil && downloadCtx.Err() != nil
	downloadCancel()
	c.downloadMu.Lock()
	c.downloadCancel = nil
	c.downloadMu.Unlock()
	if err != nil {
		// A cancelled start must not fall back to the cached config:
		// the user aborted, the core must not come up.
		if ctx.Err() != nil {
			return ErrStartCancelled
		}
		// A skipped download falls back to the cached config like a failed
		// one, but says so in the log.
		if skipped {
			c.terminal.TInfof("core.controller.config_update_skipped")
		}
		// The network backend already logs the download failure with context.
		if !c.HasCachedConfig(active.Name) {
			return errors.New("no config available")
		}
		c.terminal.TInfof("core.controller.using_existing_config")
		// A genuine download failure (not a user skip) with a cached fallback:
		// let the GUI offer a retry through the running core's proxy.
		if c.OnConfigDownloadFailed != nil && !skipped {
			c.OnConfigDownloadFailed(active.Name, err)
		}
		return nil
	}
	c.saveConfigHash(active.Name, data)
	c.cfg.SetLastUpdateFor(active.Name, time.Now())
	_ = c.cfg.Save()
	c.terminal.TInfof("core.controller.config_update_finished")
	return nil
}

// SkipConfigDownload aborts the in-flight config download of the start flow;
// the start continues with the cached config. A no-op when no download runs.
func (c *Controller) SkipConfigDownload() {
	c.downloadMu.Lock()
	cancel := c.downloadCancel
	c.downloadMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// UpdateConfigIfDue downloads the named remote config only when it is still
// due (auto-update interval elapsed, or hash mismatch when enabled) at the
// moment the config update lock is acquired. An updater that finds the
// profile already refreshed by a concurrent start flow or manual update skips
// the download and reports updated=false.
func (c *Controller) UpdateConfigIfDue(name string, autoUpdate, onHashMismatch bool) (bool, error) {
	c.configUpdateMu.Lock()
	defer c.configUpdateMu.Unlock()

	rec := c.cfg.GetConfigByName(name)
	if rec == nil || rec.IsLocal() {
		return false, nil
	}
	due := (autoUpdate && rec.ShouldUpdate()) || (onHashMismatch && c.IsConfigHashMismatch(name))
	if !due {
		return false, nil
	}

	c.terminal.TInfof("core.controller.config_updating")
	if err := c.DownloadConfigFor(name, rec.URL); err != nil {
		return false, fmt.Errorf("update failed: %w", err)
	}
	c.cfg.SetLastUpdateFor(name, time.Now())
	_ = c.cfg.Save()
	c.terminal.TInfof("core.controller.config_updated", name)
	return true, nil
}

func (c *Controller) Start() error {
	data, err := c.manager.ReadConfig()
	if err != nil {
		return err
	}
	active := c.cfg.GetActiveConfig()
	data, err = c.applyOverrides(data, active)
	if err != nil {
		return err
	}
	// Refuse early when the final config needs OS privileges the app lacks;
	// otherwise the core spawns and dies on a late, opaque tun setup error.
	if tree, terr := inboundstyle.ParseTree(data); terr == nil {
		c.setProxyAddr(inboundstyle.MixedProxyAddr(tree))
		if inboundstyle.NeedsPrivileges(tree) && !c.privileges.HasRequiredPrivileges() {
			return ErrPrivilegesRequired
		}
	}
	if err := c.manager.StartWithConfig(data); err != nil {
		return err
	}
	c.buildAPIClient()
	c.terminal.TInfof("core.controller.sing_box_started")
	return nil
}

func (c *Controller) Stop() error {
	if err := c.manager.Stop(); err != nil {
		return err
	}
	c.setProxyAddr("")
	c.terminal.TInfof("core.controller.sing_box_stopped")
	return nil
}

// SkipNextConfigUpdate makes the next start flow skip the active config
// refresh once; the start then uses the cached copy as-is.
func (c *Controller) SkipNextConfigUpdate() {
	c.skipNextRefresh.Store(true)
}

func (c *Controller) setProxyAddr(addr string) {
	c.proxyMu.Lock()
	c.proxyAddr = addr
	c.proxyMu.Unlock()
}

// CoreProxyAddr returns the listen address ("host:port") of the running
// core's mixed inbound, or "" when the core is stopped or has none.
func (c *Controller) CoreProxyAddr() string {
	c.proxyMu.Lock()
	defer c.proxyMu.Unlock()
	return c.proxyAddr
}

// CoreRelease is a pickable core release for the official-mode version list.
type CoreRelease struct {
	Version     string `json:"version"`
	Prerelease  bool   `json:"prerelease"`
	PublishedAt string `json:"publishedAt"`
}

// ListCoreVersions lists the available core releases (official source mode).
func (c *Controller) ListCoreVersions(ctx context.Context) ([]CoreRelease, error) {
	if !c.IsCoreManaged() {
		return nil, fmt.Errorf("core releases are listed only for the managed core source")
	}
	releases, err := c.manager.ListCoreReleases(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]CoreRelease, len(releases))
	for i, r := range releases {
		out[i] = CoreRelease{
			Version:     strings.TrimPrefix(r.Version, "v"),
			Prerelease:  r.Prerelease,
			PublishedAt: r.PublishedAt.Format("2006-01-02"),
		}
	}
	return out, nil
}

// DownloadCoreVersionContext installs a specific core release chosen by the
// user; the progress/install-start callbacks mirror DownloadCoreContext.
func (c *Controller) DownloadCoreVersionContext(ctx context.Context, ver string, onProgress ProgressFunc, onInstallStart func()) (string, error) {
	if c.manager.updater == nil {
		return "", fmt.Errorf("core updater not configured")
	}
	if !c.IsCoreManaged() {
		return "", fmt.Errorf("core updates are managed outside the app (core source mode)")
	}
	current, _ := GetCoreVersion(c.manager.coreBinary())
	if current != "" && !strings.HasPrefix(current, "v") {
		current = "v" + current
	}
	info, err := c.manager.updater.CheckVersion(ctx, ver, current)
	if err != nil {
		return "", err
	}
	if info.ReleaseCount == 0 {
		return "", fmt.Errorf("no matching asset in release %s", ver)
	}
	return c.installCoreUpdate(ctx, info, onProgress, onInstallStart)
}

// DownloadConfigViaCoreProxy downloads a remote profile through the running
// core's local proxy (its mixed inbound) and refreshes the cached copy. Used
// when direct access to the subscription URL is blocked but the core itself
// can reach it.
func (c *Controller) DownloadConfigViaCoreProxy(name string) error {
	addr := c.CoreProxyAddr()
	if !c.manager.IsRunning() || addr == "" {
		return c.terminal.TErrorf("core.controller.no_core_proxy")
	}
	rec := c.cfg.GetConfigByName(name)
	if rec == nil || rec.IsLocal() {
		return fmt.Errorf("config %q is not a remote profile", name)
	}

	c.configUpdateMu.Lock()
	defer c.configUpdateMu.Unlock()

	// The core process is up, but the mixed inbound may still be binding its
	// port; wait briefly for it to accept connections.
	deadline := time.Now().Add(10 * time.Second)
	for {
		conn, derr := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if derr == nil {
			_ = conn.Close()
			break
		}
		if time.Now().After(deadline) {
			return c.terminal.TErrorf("core.controller.no_core_proxy")
		}
		time.Sleep(200 * time.Millisecond)
	}

	client := fwnet.NewClientViaProxy(c.terminal, addr)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	data, err := c.manager.DownloadConfigForWith(ctx, name, rec.URL, client)
	if err != nil {
		// The network backend already logged the failure with context.
		return err
	}
	c.saveConfigHash(name, data)
	c.cfg.SetLastUpdateFor(name, time.Now())
	_ = c.cfg.Save()
	c.terminal.TInfof("core.controller.config_updated", name)
	return nil
}

func (c *Controller) Restart() error {
	c.terminal.TInfof("core.controller.restarting")
	data, err := c.manager.ReadConfig()
	if err != nil {
		return err
	}
	active := c.cfg.GetActiveConfig()
	data, err = c.applyOverrides(data, active)
	if err != nil {
		return err
	}
	if tree, terr := inboundstyle.ParseTree(data); terr == nil {
		c.setProxyAddr(inboundstyle.MixedProxyAddr(tree))
	}
	if err := c.manager.Stop(); err != nil {
		return err
	}
	for range 100 {
		if !c.manager.IsRunning() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := c.manager.StartWithConfig(data); err != nil {
		return err
	}
	c.buildAPIClient()
	c.terminal.TInfof("core.controller.sing_box_restarted")
	return nil
}

// applyOverrides applies the permanent log and API overrides to the raw
// sing-box config. It allocates a free port and secret for the API each time.
func (c *Controller) applyOverrides(data []byte, rec *config.ConfigRecord) ([]byte, error) {
	data, err := c.applyLogOverride(data)
	if err != nil {
		return nil, err
	}

	data, err = c.applyInboundsOverride(data, rec)
	if err != nil {
		return nil, err
	}

	version, _ := c.GetInstalledCoreVersion()
	port, err := api.FindFreePort("127.0.0.1")
	if err != nil {
		return nil, c.terminal.TErrorf("core.controller.find_free_api_port_failed", err)
	}
	secret := api.GenerateSecret()

	data, info, err := api.ApplyOverride(data, version, "127.0.0.1", port, secret)
	if err != nil {
		return nil, c.terminal.TErrorf("core.controller.apply_api_override_failed", err)
	}
	c.apiInfo = info
	return data, nil
}

func (c *Controller) applyInboundsOverride(data []byte, rec *config.ConfigRecord) ([]byte, error) {
	if rec == nil {
		return data, nil
	}

	tree, err := inboundstyle.ParseTree(data)
	if err != nil {
		return nil, fmt.Errorf("parse config for inbounds override: %w", err)
	}

	proxyEnabled := c.cfg.MustGet("core", "proxy", "enabled").Bool()
	fallbackType := rec.GetFallbackType()
	if err := inboundstyle.ApplyOverride(tree, proxyEnabled, fallbackType); err != nil {
		return nil, fmt.Errorf("apply inbounds override: %w", err)
	}

	output, err := inboundstyle.MarshalTree(tree)
	if err != nil {
		return nil, fmt.Errorf("marshal config after inbounds override: %w", err)
	}

	version, _ := c.GetInstalledCoreVersion()
	parser, err := singboxconfig.NewConfigParserForVersion(version)
	if err != nil {
		c.terminal.TWarnf("core.invalid_core_version", version, err)
		parser = singboxconfig.NewConfigParser()
	}
	if _, err := parser.Parse(output); err != nil {
		c.terminal.TWarnf("core.controller.config_unknown_fields_after_inbounds", err)
	}

	return output, nil
}

// DetectConfigStyle reads the cached config for the named profile and classifies
// its inbounds as client/server/undefined.
func (c *Controller) DetectConfigStyle(name string) (inboundstyle.Style, error) {
	if !c.HasCachedConfig(name) {
		return inboundstyle.StyleUndefined, fmt.Errorf("config not cached: %s", name)
	}
	data, err := c.manager.ReadConfigByName(name)
	if err != nil {
		return inboundstyle.StyleUndefined, fmt.Errorf("read config: %w", err)
	}
	tree, err := inboundstyle.ParseTree(data)
	if err != nil {
		return inboundstyle.StyleUndefined, fmt.Errorf("parse config: %w", err)
	}
	return inboundstyle.DetectFromConfig(tree), nil
}

// SetFallbackType persists the fallback type for the named profile.
func (c *Controller) SetFallbackType(name, fallbackType string) error {
	rec := c.cfg.GetConfigByName(name)
	if rec == nil {
		return fmt.Errorf("profile not found: %s", name)
	}
	updated := *rec
	updated.FallbackType = &fallbackType
	c.cfg.UpdateConfig(name, updated)
	if err := c.cfg.Save(); err != nil {
		return fmt.Errorf("save profile fallback_type: %w", err)
	}
	return nil
}

func (c *Controller) buildAPIClient() {
	// The previous client belongs to a dead core process once a new one is
	// built: close it so its transport (gRPC conn, sockets, goroutines)
	// does not leak on every start/restart cycle.
	if c.apiClient != nil {
		_ = c.apiClient.Close()
		c.apiClient = nil
	}
	if c.apiInfo == nil {
		return
	}
	switch c.apiInfo.Backend {
	case api.BackendClash:
		c.apiClient = clash.NewClient(c.apiInfo.URL(), c.apiInfo.Secret)
	case api.BackendSingBox:
		client, err := singbox.NewClient(c.apiInfo.Addr(), c.apiInfo.Secret)
		if err != nil {
			c.terminal.TWarnf("core.controller.create_api_client_failed", err)
			c.apiClient = nil
			return
		}
		c.apiClient = client
	}
}

// APIClient returns the active internal API client, or nil if the core is not
// running or the API backend is not yet supported.
func (c *Controller) APIClient() api.CoreAPIClient {
	return c.apiClient
}

// APIInfo returns the runtime connection parameters for the active API, or nil
// if the core is not running.
func (c *Controller) APIInfo() *api.Info {
	return c.apiInfo
}

func (c *Controller) IsRunning() bool {
	return c.manager.IsRunning()
}

func (c *Controller) GetPID() int {
	return c.manager.GetPID()
}

func (c *Controller) SetConfigURL(url string) {
	c.manager.SetConfigURL(url)
}

func (c *Controller) SetConfigName(name string) {
	c.manager.SetConfigName(name)
}

func (c *Controller) SetElevated(v bool) {
	c.manager.SetElevated(v)
}

func (c *Controller) UpdateConfig() error {
	_, err := c.manager.UpdateConfig(context.Background())
	return err
}

func (c *Controller) UpdateConfigNow(name, url string) error {
	rec := c.cfg.GetConfigByName(name)
	if rec != nil && rec.IsLocal() {
		return c.terminal.TErrorf("core.controller.local_config_url_update", name)
	}
	if err := c.DownloadConfigFor(name, url); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}
	c.cfg.SetLastUpdateFor(name, time.Now())
	_ = c.cfg.Save()
	c.terminal.TInfof("core.controller.config_updated", name)
	return nil
}

// ---------- Core binary ----------

func (c *Controller) CoreExists() bool {
	return c.fwApp.FS.Root().File(c.manager.coreBinary()).Exists()
}

func (c *Controller) GetInstalledCoreVersion() (string, error) {
	return GetCoreVersion(c.manager.coreBinary())
}

func (c *Controller) GetLatestCoreVersion() (string, error) {
	info, err := c.manager.CheckCoreUpdateChannel(context.Background(), c.coreChannel())
	if err != nil {
		return "", err
	}
	// The release tag carries a leading "v" ("v1.14.0") while the installed
	// version reported by the binary does not ("1.13.21") — display them in
	// the same shape.
	return strings.TrimPrefix(info.Latest, "v"), nil
}

// IsCoreManaged reports whether the core binary is managed by the app
// (official source mode) — only then do core update checks and downloads
// apply; system/custom cores are updated outside the app.
func (c *Controller) IsCoreManaged() bool {
	m := c.cfg.MustGet("core", "source", "mode").String()
	return m == "" || m == CoreSourceOfficial
}

// coreChannel returns the configured release channel for the managed core
// (core.source.channel), defaulting to stable.
func (c *Controller) coreChannel() string {
	ch := c.cfg.MustGet("core", "source", "channel").String()
	if ch == "" {
		return CoreChannelStable
	}
	return ch
}

// CheckCoreUpdateViaProxy checks for a core update routing the request
// through the local proxy at proxyAddr ("host:port") — typically the running
// core's mixed inbound, used when direct access to the release source is
// blocked. Returns the latest version like GetLatestCoreVersion.
func (c *Controller) CheckCoreUpdateViaProxy(proxyAddr string) (string, error) {
	src := c.manager.updater
	if src == nil {
		return "", fmt.Errorf("core updater not configured")
	}
	gh, ok := src.Source.(*updater.GitHubBackend)
	if !ok {
		return "", fmt.Errorf("core updater source does not support proxying")
	}
	proxied := *src
	proxiedGH := &updater.GitHubBackend{
		BaseURL: gh.BaseURL,
		Owner:   gh.Owner,
		Repo:    gh.Repo,
		Net:     fwnet.NewClientViaProxy(c.terminal, proxyAddr),
		Log:     gh.Log,
	}
	proxied.Source = proxiedGH
	current, _ := GetCoreVersion(c.manager.coreBinary())
	if current != "" && !strings.HasPrefix(current, "v") {
		current = "v" + current
	}
	if c.coreChannel() == CoreChannelBeta {
		releases, err := proxiedGH.ListReleases(context.Background())
		if err != nil {
			return "", err
		}
		if len(releases) == 0 {
			return strings.TrimPrefix(current, "v"), nil
		}
		info, err := proxied.CheckVersion(context.Background(), releases[0].Version, current)
		if err != nil {
			return "", err
		}
		return info.Latest, nil
	}
	info, err := proxied.CheckWithCurrent(context.Background(), "", current)
	if err != nil {
		return "", err
	}
	return info.Latest, nil
}

func (c *Controller) DownloadCoreWithProgress(onProgress func(downloaded, total int64)) (string, error) {
	return c.DownloadCore(onProgress)
}

func (c *Controller) DownloadCore(onProgress ProgressFunc) (string, error) {
	return c.DownloadCoreContext(context.Background(), onProgress, nil)
}

// DownloadCoreContext is DownloadCore with a caller-supplied context so the
// GUI can cancel an in-flight download. onInstallStart, when non-nil, is
// called once the download finishes and the binary replacement begins.
func (c *Controller) DownloadCoreContext(ctx context.Context, onProgress ProgressFunc, onInstallStart func()) (string, error) {
	if c.manager.updater == nil {
		return "", fmt.Errorf("core updater not configured")
	}
	if !c.IsCoreManaged() {
		return "", fmt.Errorf("core updates are managed outside the app (core source mode)")
	}

	info, err := c.manager.CheckCoreUpdateChannel(ctx, c.coreChannel())
	if err != nil {
		return "", err
	}
	if info.ReleaseCount == 0 {
		c.terminal.TInfof("core.controller.core_up_to_date", info.Current)
		return c.manager.coreBinary(), nil
	}
	c.terminal.TInfof("core.controller.latest_core_version", info.Latest)

	return c.installCoreUpdate(ctx, info, onProgress, onInstallStart)
}

// installCoreUpdate downloads and installs the core release described by
// info, stopping a running core right before the binary replacement and
// starting it again afterwards. onInstallStart, when non-nil, is called once
// the download finishes and the replacement begins.
func (c *Controller) installCoreUpdate(ctx context.Context, info *updater.UpdateInfo, onProgress ProgressFunc, onInstallStart func()) (string, error) {
	info.Files = []updater.UpdateFile{{
		Asset:    info.Asset,
		DestPath: ".",
	}}

	wasRunning := c.manager.IsRunning()

	// Stop the running core right before its binary is replaced. The download
	// can run while the core is still active; only the actual installation must
	// happen on a stopped process.
	if fa, ok := c.manager.updater.Apply.(*updater.FilesUpdateApply); ok {
		fa.BeforeInstall = func() error {
			// The download is complete; the binary replacement begins here.
			if onInstallStart != nil {
				onInstallStart()
			}
			if !c.manager.IsRunning() {
				return nil
			}
			c.terminal.TInfof("core.controller.stopping_core_for_update")
			if err := c.manager.Stop(); err != nil {
				return fmt.Errorf("stop core for update: %w", err)
			}
			for range 100 {
				if !c.manager.IsRunning() {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			if c.manager.IsRunning() {
				return fmt.Errorf("core process did not stop in time for update")
			}
			return nil
		}
		defer func() { fa.BeforeInstall = nil }()
	}

	if err := c.manager.updater.Install(ctx, info, onProgress); err != nil {
		return "", fmt.Errorf("install core update: %w", err)
	}

	if wasRunning {
		c.terminal.TInfof("core.manager.restarting_after_update")
		if err := c.Start(); err != nil {
			c.terminal.TInfof("core.manager.restart_after_update_failed", err)
		}
	}

	c.terminal.TInfof("core.controller.core_downloaded", c.manager.coreBinary())
	return c.manager.coreBinary(), nil
}

func (c *Controller) HasCachedConfig(name string) bool {
	return c.fwApp.FS.Root().File(c.manager.cachedConfig(name)).Exists()
}

func (c *Controller) DownloadConfigFor(name, url string) error {
	data, err := c.manager.DownloadConfigFor(context.Background(), name, url)
	if err != nil {
		return err
	}
	c.saveConfigHash(name, data)
	return nil
}

func (c *Controller) saveConfigHash(name string, data []byte) {
	rec := c.cfg.GetConfigByName(name)
	if rec == nil {
		return
	}
	hash := config.HashConfig(data)
	if rec.Hash == hash {
		return
	}
	rec.Hash = hash
	c.cfg.UpdateConfig(name, *rec)
	_ = c.cfg.Save()
}

// IsConfigHashMismatch reports whether the cached config content differs from
// the hash stored in the profile. If no hash is stored, it returns false.
func (c *Controller) IsConfigHashMismatch(name string) bool {
	rec := c.cfg.GetConfigByName(name)
	if rec == nil {
		return false
	}
	if rec.Hash == "" || !c.HasCachedConfig(name) {
		return false
	}
	data, err := c.manager.ReadConfigByName(name)
	if err != nil {
		return false
	}
	return rec.Hash != config.HashConfig(data)
}

func (c *Controller) AddConfig(rec config.ConfigRecord) error {
	if rec.Name == "" {
		return c.terminal.TErrorf("core.controller.name_required")
	}
	if !rec.IsLocal() && rec.URL == "" {
		return c.terminal.TErrorf("core.controller.url_required_remote")
	}
	if c.cfg.GetConfigByName(rec.Name) != nil {
		return c.terminal.TErrorf("core.controller.config_already_exists")
	}
	if rec.IsLocal() {
		if rec.Type == "" {
			rec.Type = config.ConfigTypeLocal
		}
		if err := c.manager.CreateLocalConfig(rec.Name); err != nil {
			return fmt.Errorf("failed to create local config: %w", err)
		}
		rec.Hash = config.HashConfig([]byte("{}"))
	}
	c.cfg.AddConfig(rec)
	if c.cfg.GetActiveName() == "" {
		c.cfg.SetActiveName(rec.Name)
		c.manager.SetConfigURL(rec.URL)
		c.manager.SetConfigName(rec.Name)
	}
	_ = c.cfg.Save()
	c.terminal.TInfof("core.controller.config_added", rec.Name)
	return nil
}

func (c *Controller) EditConfig(oldName string, rec config.ConfigRecord) error {
	if rec.Name == "" {
		return c.terminal.TErrorf("core.controller.name_required")
	}
	if !rec.IsLocal() && rec.URL == "" {
		return c.terminal.TErrorf("core.controller.url_required_remote")
	}
	oldRec := c.cfg.GetConfigByName(oldName)
	if oldRec != nil {
		// The edit form carries only its own fields; runtime-owned state
		// (content hash, last update, auto-update flag, fallback choice,
		// plugin parent) is preserved from the stored record so editing
		// e.g. the update interval does not reset the cache state.
		rec.Hash = oldRec.Hash
		rec.LastUpdate = oldRec.LastUpdate
		rec.AutoUpdate = oldRec.AutoUpdate
		rec.FallbackType = oldRec.FallbackType
		if rec.Parent == "" {
			rec.Parent = oldRec.Parent
		}
	}
	if rec.Name != oldName {
		if c.cfg.GetConfigByName(rec.Name) != nil {
			return c.terminal.TErrorf("core.controller.config_name_already_exists", rec.Name)
		}
		if rec.IsLocal() {
			_ = c.manager.RenameConfigFile(oldName, rec.Name)
		}
		c.cfg.RenameConfig(oldName, rec.Name)
	}
	c.cfg.UpdateConfig(rec.Name, rec)
	_ = c.cfg.Save()
	if oldName == c.cfg.GetActiveName() || rec.Name == c.cfg.GetActiveName() {
		c.manager.SetConfigURL(rec.URL)
	}
	if rec.Name != oldName {
		c.terminal.TInfof("core.controller.config_renamed", oldName, rec.Name)
	} else {
		c.terminal.TInfof("core.controller.config_updated", rec.Name)
	}
	return nil
}

func (c *Controller) DeleteConfig(name string) error {
	c.cfg.RemoveConfig(name)
	_ = c.cfg.Save()
	if c.cfg.MustGet("configs", "delete_cache_on_remove").Bool() {
		// The scoped FS logs removal failures itself.
		if err := c.manager.DeleteConfigFile(name); err != nil {
			return fmt.Errorf("delete cached config file: %w", err)
		}
	}
	c.terminal.TInfof("core.controller.config_deleted", name)
	return nil
}

// DeleteCachedConfig removes the cached config file of the named profile or
// orphan without touching the profiles list.
func (c *Controller) DeleteCachedConfig(name string) error {
	return c.manager.DeleteConfigFile(name)
}

// OrphanedConfig describes a cached config file with no matching profile.
type OrphanedConfig struct {
	Name string `json:"name"`
	// ModTime is the file modification time on disk (RFC3339).
	ModTime string `json:"modTime"`
}

// OrphanedConfigs lists cached config files that have no matching profile —
// leftovers from deleted profiles.
func (c *Controller) OrphanedConfigs() []OrphanedConfig {
	entries, err := c.fwApp.FS.Root().Subdir("configs").ReadDir()
	if err != nil {
		return nil
	}
	var out []OrphanedConfig
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		profile := strings.TrimSuffix(name, ".json")
		if c.cfg.GetConfigByName(profile) != nil {
			continue
		}
		o := OrphanedConfig{Name: profile}
		if fi, err := e.Stat(); err == nil {
			o.ModTime = fi.ModTime().Format(time.RFC3339)
		}
		out = append(out, o)
	}
	return out
}

func (c *Controller) ActivateConfig(name string) error {
	rec := c.cfg.GetConfigByName(name)
	if rec == nil {
		return fmt.Errorf("config not found")
	}
	if !c.HasCachedConfig(name) {
		if rec.IsLocal() {
			if err := c.manager.CreateLocalConfig(name); err != nil {
				return fmt.Errorf("failed to create local config: %w", err)
			}
		} else {
			return c.terminal.TErrorf("core.controller.no_cached_config", name)
		}
	}
	c.cfg.SetActiveName(name)
	_ = c.cfg.Save()
	c.manager.SetConfigURL(rec.URL)
	c.manager.SetConfigName(name)
	c.terminal.TInfof("core.controller.config_activated", name)
	return nil
}

func (c *Controller) UpdateAllConfigs(progress func(done, total int)) (int, int, error) {
	configs := c.cfg.GetConfigs()
	total := 0
	for _, rec := range configs {
		if !rec.IsLocal() {
			total++
		}
	}
	if total == 0 {
		c.terminal.TInfof("core.controller.no_configs_to_update")
		return 0, 0, nil
	}
	updated := 0
	for _, rec := range configs {
		if rec.IsLocal() {
			continue
		}
		c.terminal.TInfof("core.controller.updating_config", rec.Name)
		if err := c.DownloadConfigFor(rec.Name, rec.URL); err != nil {
			// The underlying I/O error is already logged by the scoped FS;
			// continue with the remaining configs.
			continue
		} else {
			c.cfg.SetLastUpdateFor(rec.Name, time.Now())
			updated++
			c.terminal.TInfof("core.controller.config_updated", rec.Name)
		}
		if progress != nil {
			progress(updated, total)
		}
	}
	_ = c.cfg.Save()
	c.terminal.TInfof("core.controller.update_all_finished", updated, total)
	return updated, total, nil
}

// OpenConfigFile opens the cached config file in the platform default editor.
func (c *Controller) OpenConfigFile(name string) error {
	if !c.HasCachedConfig(name) {
		return c.terminal.TErrorf("core.controller.config_file_not_found", name)
	}
	path := c.manager.cachedConfig(name)
	c.terminal.TInfof("core.controller.opening_config_file", path)
	return openfile.OpenPath(path)
}

// ValidateConfig reads the cached config and runs a deprecation/validation check
// against the installed sing-box core version.
func (c *Controller) ValidateConfig(name string) (singboxconfig.ValidationResult, error) {
	if !c.HasCachedConfig(name) {
		return singboxconfig.ValidationResult{}, c.terminal.TErrorf("core.controller.config_file_not_found", name)
	}
	path := c.manager.cachedConfig(name)
	data, err := c.fwApp.FS.Root().File(path).Read()
	if err != nil {
		return singboxconfig.ValidationResult{}, fmt.Errorf("failed to read config file: %w", err)
	}
	rec := c.cfg.GetConfigByName(name)
	if rec == nil {
		return singboxconfig.ValidationResult{}, fmt.Errorf("profile not found: %s", name)
	}
	data, err = c.applyOverrides(data, rec)
	if err != nil {
		return singboxconfig.ValidationResult{}, err
	}

	var parser *singboxconfig.ConfigParser
	if version, err := c.GetInstalledCoreVersion(); err == nil && version != "" {
		parser, err = singboxconfig.NewConfigParserForVersion(version)
		if err != nil {
			c.terminal.TWarnf("core.invalid_core_version", version, err)
			parser = singboxconfig.NewConfigParser()
		}
	} else {
		parser = singboxconfig.NewConfigParser()
	}

	if _, err := parser.Parse(data); err != nil {
		result := parser.Result()
		return result, err
	}
	result := parser.Result()
	return result, nil
}

// OpenConfigDir opens the directory containing the cached config file.
func (c *Controller) OpenConfigDir(name string) error {
	if !c.HasCachedConfig(name) {
		return c.terminal.TErrorf("core.controller.config_file_not_found", name)
	}
	path := c.manager.cachedConfig(name)
	dir := filepath.Dir(path)
	c.terminal.TInfof("core.controller.opening_config_dir", dir)
	return openfile.OpenPath(dir)
}

// RecreateLocalConfig recreates the local config file if it is missing.
func (c *Controller) RecreateLocalConfig(name string) error {
	rec := c.cfg.GetConfigByName(name)
	if rec == nil {
		return fmt.Errorf("config not found")
	}
	if !rec.IsLocal() {
		return c.terminal.TErrorf("core.controller.only_local_recreate")
	}
	if err := c.manager.CreateLocalConfig(name); err != nil {
		return fmt.Errorf("failed to recreate local config: %w", err)
	}
	c.terminal.TInfof("core.controller.recreated_local_config", name)
	return nil
}

// ConfigNameAvailable reports whether name is free for a new profile.
// exclude names the profile being edited, which may keep its own name.
func (c *Controller) ConfigNameAvailable(name, exclude string) bool {
	if name == "" {
		return false
	}
	if name == exclude {
		return true
	}
	return c.cfg.GetConfigByName(name) == nil
}

// LocalConfigModTime returns the modification time of the cached config file
// of a local profile, which acts as its "last updated" timestamp.
func (c *Controller) LocalConfigModTime(name string) (time.Time, error) {
	fi, err := c.fwApp.FS.Root().File(c.manager.cachedConfig(name)).Stat()
	if err != nil {
		return time.Time{}, fmt.Errorf("stat config file: %w", err)
	}
	return fi.ModTime(), nil
}

// ImportLocalConfigFile replaces the cached content of a local profile with
// the contents of a user-picked file and stores its content hash.
func (c *Controller) ImportLocalConfigFile(name, sourcePath string) error {
	rec := c.cfg.GetConfigByName(name)
	if rec == nil {
		return c.terminal.TErrorf("core.controller.import_config_not_found", name)
	}
	if !rec.IsLocal() {
		return c.terminal.TErrorf("core.controller.only_local_import")
	}
	// #nosec G304 — the path comes from the user's own file picker.
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return c.terminal.TErrorf("core.controller.import_read_failed", err)
	}
	if !json.Valid(data) {
		return c.terminal.TErrorf("core.controller.import_not_json", sourcePath)
	}
	if err := c.fwApp.FS.Root().Subdir("configs").MkdirAll(0750); err != nil {
		return err
	}
	// The scoped FS logs write failures itself.
	if err := c.fwApp.FS.Root().File(c.manager.cachedConfig(name)).AtomicWrite(data, 0640); err != nil {
		return err
	}
	rec.Hash = config.HashConfig(data)
	c.cfg.UpdateConfig(name, *rec)
	_ = c.cfg.Save()
	c.terminal.TInfof("core.controller.local_config_imported", name)
	return nil
}

// ---------- Privileges ----------

func (c *Controller) HasRequiredPrivileges() bool {
	return c.privileges.HasRequiredPrivileges()
}

func (c *Controller) RefreshPrivilegeStatus() string {
	return c.privileges.RefreshPrivilegeStatus()
}

func (c *Controller) GetPrivilegeDialog() *PrivilegeDialog {
	return c.privileges.GetPrivilegeDialog(c.RestartAsAdmin)
}

func (c *Controller) GetPrivilegeTabState() PrivilegeTabState {
	return c.privileges.GetPrivilegeTabState()
}

func (c *Controller) RestartAsAdmin() error {
	if runtime.GOOS != "windows" {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// Preserve the current command-line arguments in the elevated restart.
	var argList string
	if len(os.Args) > 1 {
		quoted := make([]string, len(os.Args)-1)
		for i, a := range os.Args[1:] {
			quoted[i] = strconv.Quote(a)
		}
		argList = "@(" + strings.Join(quoted, ",") + ")"
	}

	script := fmt.Sprintf("Start-Process -FilePath %q -Verb runAs -WorkingDirectory %q", exe, cwd)
	if argList != "" {
		script += fmt.Sprintf(" -ArgumentList %s", argList)
	}

	// #nosec G204 — powershell is a system binary; exe and cwd come from the
	// running process, and arguments are preserved from os.Args.
	cmd := exec.Command("powershell", "-WindowStyle", "hidden", "-Command", script)
	if err := cmd.Start(); err != nil {
		return err
	}

	// The elevated copy is being started; terminate the current unprivileged
	// instance so only one copy of the application remains running.
	os.Exit(0)
	return nil
}

func (c *Controller) ApplySetcap() error {
	if err := SetNetAdminCapabilityGUI(c.manager.coreBinary()); err != nil {
		return c.terminal.TErrorf("core.controller.setcap_failed", err)
	}
	c.terminal.TInfof("core.controller.setcap_applied")
	return nil
}

// ToggleSetcap applies or removes cap_net_admin on the core binary depending
// on the current state. It returns a plain error; the caller logs (e.g. via
// ApplyPrivilegeAction).
func (c *Controller) ToggleSetcap() error {
	return c.privileges.ToggleSetcap()
}

func (c *Controller) ApplyPrivilegeAction(action *PrivilegeAction) (success, needRefresh, needClose bool) {
	return c.privileges.ApplyPrivilegeAction(action)
}

// ---------- App helpers ----------

func (c *Controller) OpenDataDir() error {
	if c.fwApp == nil {
		return nil
	}
	return c.fwApp.OpenDataDir()
}

func (c *Controller) SetLogLimit(v int) {
	_ = c.cfg.MustGet("log", "limit").Update(v)
	_ = c.cfg.Save()
	c.fwApp.Logger.SetLimit(v)
	c.processor.LogBuffer().SetLimit(v)
	c.terminal.TInfof("core.controller.log_limit_set", v)
}

func (c *Controller) SetDefaultInterval(h int) {
	_ = c.cfg.MustGet("updates", "default_interval_hours").Update(h)
	_ = c.cfg.Save()
	c.terminal.TInfof("core.controller.default_interval_set", h)
}

func (c *Controller) SetAutoRestart(checked bool) error {
	_ = c.cfg.MustGet("core", "auto_restart").Update(checked)
	if err := c.cfg.Save(); err != nil {
		return fmt.Errorf("failed to save auto-restart setting: %w", err)
	}
	c.terminal.TInfof("core.controller.auto_restart", checked)
	return nil
}
