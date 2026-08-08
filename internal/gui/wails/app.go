//go:build !nogui

package wails

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/he11ah0und/localengine"
	apppkg "sing-box-ez/internal/app"
	"sing-box-ez/internal/app/themes"
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
	"sing-box-ez/internal/core/inboundstyle"
	"sing-box-ez/internal/core/state"
	"sing-box-ez/internal/framework/version"
	"sing-box-ez/internal/gui/tray"
)

// Status holds a snapshot of the core runtime state.
type Status struct {
	Running bool `json:"running"`
	PID     int  `json:"pid"`
}

// CoreInfo holds version and update state for the sing-box core.
type CoreInfo struct {
	InstalledVersion string `json:"installedVersion"`
	LatestVersion    string `json:"latestVersion"`
	Downloading      bool   `json:"downloading"`
	DownloadProgress int    `json:"downloadProgress"`
}

// Settings is the UI-facing representation of application settings.
type Settings struct {
	Language             string `json:"language"`
	Theme                string `json:"theme"`
	ThemeMode            string `json:"themeMode"`
	AutoStartCore        bool   `json:"autoStartCore"`
	AutoRestart          bool   `json:"autoRestart"`
	RunAsAdmin           bool   `json:"runAsAdmin"`
	LogLimit             int    `json:"logLimit"`
	DesktopNotifications bool   `json:"desktopNotifications"`
	AutoCheckCore        bool   `json:"autoCheckCore"`
	AutoCheckSelf        bool   `json:"autoCheckSelf"`
	DefaultIntervalHours int    `json:"defaultIntervalHours"`

	ProxyEnabled        bool   `json:"proxyEnabled"`
	URLTestURL          string `json:"urlTestURL"`
	CoreLogLevel        string `json:"coreLogLevel"`
	TrafficGraphHistory int    `json:"trafficGraphHistory"`
	ShowLogs            bool   `json:"showLogs"`

	AutoUpdateConfigs                  bool `json:"autoUpdateConfigs"`
	AutoUpdateConfigsIntervalHours     int  `json:"autoUpdateConfigsIntervalHours"`
	AutoUpdateOnHashMismatch           bool `json:"autoUpdateOnHashMismatch"`
	AutoRestartOnConfigUpdate          bool `json:"autoRestartOnConfigUpdate"`
	BackgroundUpdateCheckIntervalHours int  `json:"backgroundUpdateCheckIntervalHours"`
}

// ThemePayload is the UI-facing representation of the active theme.
type ThemePayload struct {
	Name   string            `json:"name"`
	Mode   string            `json:"mode"`
	Colors map[string]string `json:"colors"`
}

// Locale is the UI-facing representation of the active locale.
type Locale struct {
	Language string            `json:"language"`
	Values   map[string]string `json:"values"`
}

// LanguageOption describes an available language for the UI picker.
type LanguageOption struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// Bindings exposes Go methods that can be called from the frontend.
type Bindings struct {
	ctx                context.Context
	app                *apppkg.App
	ws                 *wsServer
	ic                 *core.InteractiveController
	theme              *themes.Collection
	localeKeys         map[string]struct{}
	// localeWildcards holds registered wildcard prefixes (without the ".*"
	// suffix); they expand to all matching leaf paths of the current language.
	localeWildcards      map[string]struct{}
	localeReady        bool
	localeMu           sync.Mutex

	// core is the background core state poller holding the graph history and
	// the last known API state (phase, groups, connections).
	core *state.Poller

	// corePhaseHint holds the current core lifecycle stage reported by the
	// start/stop flow (see the Phase* constants in internal/core/state), so
	// the poller can show granular progress phases. Empty when idle.
	corePhaseHint atomic.Value // string

	// styleCheckMu guards pendingStyleChecks.
	styleCheckMu sync.Mutex
	// pendingStyleChecks holds the choose callbacks of in-flight config style
	// checks, keyed by profile name. A SetFallbackType call for the profile
	// consumes the callback and resumes the start flow.
	pendingStyleChecks map[string]func(string)
}

// ServiceName returns the service name used by Wails.
func (b *Bindings) ServiceName() string {
	return "App"
}

// ServiceStartup is called by Wails when the application starts.
func (b *Bindings) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	b.ctx = ctx
	return nil
}

// GetStatus returns whether the sing-box core is running.
func (b *Bindings) GetStatus() Status {
	return Status{
		Running: b.app.Controller.IsRunning(),
		PID:     b.app.Controller.GetPID(),
	}
}

// GetConfigs returns all configured profiles.
func (b *Bindings) GetConfigs() []config.ConfigRecord {
	return b.app.Controller.GetConfigs()
}

// GetActiveConfig returns the currently active profile, or nil.
// ActiveConfig pairs the active profile with UI-facing computed fields.
type ActiveConfig struct {
	*config.ConfigRecord
	// LastUpdateAgo is the localized human-readable form of the elapsed time
	// since the profile was last updated.
	LastUpdateAgo string `json:"lastUpdateAgo"`
}

// GetActiveConfig returns the active profile.
func (b *Bindings) GetActiveConfig() *ActiveConfig {
	rec := b.app.Controller.GetActiveConfig()
	if rec == nil {
		return nil
	}
	return &ActiveConfig{
		ConfigRecord:  rec,
		LastUpdateAgo: version.HumanDuration(rec.LastUpdate.Time),
	}
}

// ActivateConfig sets the active profile by name.
func (b *Bindings) ActivateConfig(name string) error {
	if err := b.app.Controller.ActivateConfig(name); err != nil {
		b.toastErr(err)
		return err
	}
	return nil
}

// AddConfig creates a new profile.
func (b *Bindings) AddConfig(rec config.ConfigRecord) error {
	if err := b.app.Controller.AddConfig(rec); err != nil {
		b.toastErr(err)
		return err
	}
	return nil
}

// EditConfig updates an existing profile.
func (b *Bindings) EditConfig(oldName string, rec config.ConfigRecord) error {
	if err := b.app.Controller.EditConfig(oldName, rec); err != nil {
		b.toastErr(err)
		return err
	}
	return nil
}

// DeleteConfig removes a profile by name.
func (b *Bindings) DeleteConfig(name string) error {
	if err := b.app.Controller.DeleteConfig(name); err != nil {
		b.toastErr(err)
		return err
	}
	return nil
}

// Start prepares the active config and starts the sing-box core. It goes
// through the interactive start flow (config refresh, hash-mismatch handling,
// client-style check) so the button behaves like the legacy UI and the tray.
func (b *Bindings) Start() error {
	b.setPhaseHint(state.PhasePreparingConfig)
	defer b.setPhaseHint("")
	var err error
	if b.ic == nil {
		err = b.app.Controller.Start()
	} else {
		err = b.ic.StartService()
	}
	if err != nil {
		b.toastErr(err, "main", "btn", "start")
	}
	return err
}

// setPhaseHint stores the current core lifecycle stage for the state poller.
func (b *Bindings) setPhaseHint(phase string) {
	b.corePhaseHint.Store(phase)
}

// phaseHint returns the current core lifecycle stage, or "" when idle.
func (b *Bindings) phaseHint() string {
	if v := b.corePhaseHint.Load(); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// Stop stops the sing-box core.
func (b *Bindings) Stop() error {
	b.setPhaseHint(state.PhaseStopping)
	defer b.setPhaseHint("")
	if err := b.app.Controller.Stop(); err != nil {
		b.toastErr(err, "main", "btn", "stop")
		return err
	}
	return nil
}

// Restart restarts the sing-box core.
func (b *Bindings) Restart() error {
	b.setPhaseHint(state.PhaseStarting)
	defer b.setPhaseHint("")
	if err := b.app.Controller.Restart(); err != nil {
		b.toastErr(err, "main", "btn", "restart")
		return err
	}
	return nil
}

// GetCoreInfo returns installed and latest core version information.
func (b *Bindings) GetCoreInfo() CoreInfo {
	info := CoreInfo{}
	if v, err := b.app.Controller.GetInstalledCoreVersion(); err == nil {
		info.InstalledVersion = v
	}
	if v, err := b.app.Controller.GetLatestCoreVersion(); err == nil {
		info.LatestVersion = v
	}
	return info
}

// DownloadCore downloads and installs the latest sing-box core.
func (b *Bindings) DownloadCore() error {
	_, err := b.app.Controller.DownloadCoreWithProgress(func(downloaded, total int64) {
		progress := 0
		if total > 0 {
			progress = int(float64(downloaded) / float64(total) * 100)
		}
		b.emit("update:progress", map[string]int64{
			"downloaded": downloaded,
			"total":      total,
			"progress":   int64(progress),
		})
	})
	return err
}

// GetLocale returns the currently active locale and registered key values.
func (b *Bindings) GetLocale() Locale {
	b.localeMu.Lock()
	defer b.localeMu.Unlock()
	return Locale{
		Language: localengine.CurrentLanguage(),
		Values:   b.localeKeyValuesLocked(),
	}
}

// GetAvailableLanguages returns the list of languages available for the UI.
func (b *Bindings) GetAvailableLanguages() []LanguageOption {
	codes := localengine.AvailableLanguages()
	out := make([]LanguageOption, 0, len(codes))
	for _, code := range codes {
		out = append(out, LanguageOption{
			Code: code,
			Name: localengine.LanguageName(code),
		})
	}
	return out
}

// SetLanguage changes the active language, persists it to config and notifies the UI.
func (b *Bindings) SetLanguage(code string) Locale {
	_ = b.app.Controller.Config().MustGet("ui", "language").Update(code)
	_ = b.app.Controller.Config().Save()
	localengine.SetLanguage(code)
	l := b.GetLocale()
	b.emit("locale:changed", l)
	b.emitLocaleKeyValues()
	return l
}

// RegisterLocaleKeys registers UI locale keys and returns their current values.
// A key ending in ".*" is a wildcard: it registers every leaf path under
// that prefix (expanded per current language, so language switches
// re-expand it). Missing keys are reported by the localengine lookup
// itself when the values are resolved below.
func (b *Bindings) RegisterLocaleKeys(keys []string) map[string]string {
	b.localeMu.Lock()
	defer b.localeMu.Unlock()
	for _, k := range keys {
		if prefix, ok := strings.CutSuffix(k, ".*"); ok {
			if _, known := b.localeWildcards[prefix]; !known {
				b.localeWildcards[prefix] = struct{}{}
				if !b.wildcardMatchesLocked(prefix) {
					b.app.Logger.Root.Warnf("locale wildcard %q matched no keys", k)
				}
			}
			continue
		}
		b.localeKeys[k] = struct{}{}
	}
	return b.localeKeyValuesLocked()
}

// LocaleReady tells the backend that the initial set of UI keys is registered.
// It refreshes registered values so late subscribers get them in one event.
func (b *Bindings) LocaleReady() {
	b.localeMu.Lock()
	b.localeReady = true
	values := b.localeKeyValuesLocked()
	b.localeMu.Unlock()
	if len(values) > 0 {
		b.emit("locale:keys_changed", values)
	}
}

func (b *Bindings) emitLocaleKeyValues() {
	b.localeMu.Lock()
	values := b.localeKeyValuesLocked()
	b.localeMu.Unlock()
	if len(values) == 0 {
		return
	}
	b.emit("locale:keys_changed", values)
}

func (b *Bindings) localeKeyValuesLocked() map[string]string {
	values := make(map[string]string, len(b.localeKeys))
	lang := localengine.CurrentLanguage()
	for k := range b.localeKeys {
		if v, ok := localengine.LookupString(lang, strings.Split(k, ".")...); ok {
			values[k] = v
		}
	}
	for prefix := range b.localeWildcards {
		// LeafPaths does not cover static bundles, so wildcards only expand
		// over real translations (current language, then English for keys the
		// language lacks). LookupString resolves the value with the usual
		// language → en → static order.
		seen := make(map[string]struct{})
		for _, code := range []string{lang, "en"} {
			for _, p := range localengine.LeafPaths(code) {
				if !strings.HasPrefix(p, prefix+".") {
					continue
				}
				if _, dup := seen[p]; dup {
					continue
				}
				seen[p] = struct{}{}
				if v, ok := localengine.LookupString(lang, strings.Split(p, ".")...); ok {
					values[p] = v
				}
			}
		}
	}
	return values
}

// wildcardMatchesLocked reports whether any leaf path of the current
// language or English sits under the wildcard prefix.
func (b *Bindings) wildcardMatchesLocked(prefix string) bool {
	for _, code := range []string{localengine.CurrentLanguage(), "en"} {
		for _, p := range localengine.LeafPaths(code) {
			if strings.HasPrefix(p, prefix+".") {
				return true
			}
		}
	}
	return false
}

// GetThemeNames returns the sorted list of available theme names.
func (b *Bindings) GetThemeNames() []string {
	if b.theme == nil {
		return nil
	}
	return b.theme.Names()
}

// GetTheme returns the currently active theme with resolved colors.
func (b *Bindings) GetTheme() ThemePayload {
	return b.themePayload()
}

func (b *Bindings) themePayload() ThemePayload {
	cfg := b.app.Controller.Config()
	name := cfg.MustGet("ui", "theme").String()
	mode := cfg.MustGet("ui", "theme_mode").String()
	return ThemePayload{
		Name:   name,
		Mode:   mode,
		Colors: b.resolveThemeColors(name, mode),
	}
}

func (b *Bindings) resolveThemeColors(name, mode string) map[string]string {
	isDark := false
	if app := application.Get(); app != nil && app.Env != nil {
		isDark = app.Env.IsDarkMode()
	}
	if b.theme != nil {
		return b.theme.Resolve(name, mode, isDark)
	}
	return nil
}

func (b *Bindings) emitTheme() {
	b.emit("theme:changed", b.themePayload())
}

func (b *Bindings) applyWindowBackground(win application.Window) {
	colors := b.resolveThemeColors(
		b.app.Controller.Config().MustGet("ui", "theme").String(),
		b.app.Controller.Config().MustGet("ui", "theme_mode").String(),
	)
	if colors == nil {
		return
	}
	bg, ok := colors["bg"]
	if !ok {
		return
	}
	c, err := themes.ParseHex(bg)
	if err != nil {
		return
	}
	win.SetBackgroundColour(application.NewRGB(c.R, c.G, c.B))
}

// GetSettings returns the current application settings.
func (b *Bindings) GetSettings() Settings {
	cfg := b.app.Controller.Config()
	return Settings{
		Language:             cfg.MustGet("ui", "language").String(),
		Theme:                cfg.MustGet("ui", "theme").String(),
		ThemeMode:            cfg.MustGet("ui", "theme_mode").String(),
		AutoStartCore:        cfg.MustGet("core", "start_on_launch").Bool(),
		AutoRestart:          cfg.MustGet("core", "auto_restart").Bool(),
		RunAsAdmin:           cfg.MustGet("privileges", "run_as_admin").Bool(),
		LogLimit:             cfg.MustGet("log", "limit").Int(),
		DesktopNotifications: cfg.MustGet("ui", "desktop_notifications").Bool(),
		AutoCheckCore:        cfg.MustGet("updates", "auto_check_core").Bool(),
		AutoCheckSelf:        cfg.MustGet("updates", "auto_check_self").Bool(),
		DefaultIntervalHours: cfg.MustGet("updates", "default_interval_hours").Int(),

		ProxyEnabled:        cfg.MustGet("core", "proxy", "enabled").Bool(),
		URLTestURL:          cfg.MustGet("core", "url_test_url").String(),
		CoreLogLevel:        cfg.MustGet("core", "log", "level").String(),
		TrafficGraphHistory: cfg.MustGet("core", "traffic_graph_history").Int(),
		ShowLogs:            cfg.MustGet("ui", "show_logs").Bool(),

		AutoUpdateConfigs:                  cfg.MustGet("updates", "auto_update_configs").Bool(),
		AutoUpdateConfigsIntervalHours:     cfg.MustGet("updates", "auto_update_configs_interval_hours").Int(),
		AutoUpdateOnHashMismatch:           cfg.MustGet("updates", "auto_update_on_hash_mismatch").Bool(),
		AutoRestartOnConfigUpdate:          cfg.MustGet("updates", "auto_restart_on_config_update").Bool(),
		BackgroundUpdateCheckIntervalHours: cfg.MustGet("updates", "background_update_check_interval_hours").Int(),
	}
}

// SaveSettings persists the provided settings.
func (b *Bindings) SaveSettings(s Settings) error {
	cfg := b.app.Controller.Config()
	_ = cfg.MustGet("ui", "language").Update(s.Language)
	_ = cfg.MustGet("ui", "theme").Update(s.Theme)
	_ = cfg.MustGet("ui", "theme_mode").Update(s.ThemeMode)
	_ = cfg.MustGet("core", "start_on_launch").Update(s.AutoStartCore)
	_ = cfg.MustGet("core", "auto_restart").Update(s.AutoRestart)
	_ = cfg.MustGet("privileges", "run_as_admin").Update(s.RunAsAdmin)
	_ = cfg.MustGet("log", "limit").Update(s.LogLimit)
	_ = cfg.MustGet("ui", "desktop_notifications").Update(s.DesktopNotifications)
	_ = cfg.MustGet("updates", "auto_check_core").Update(s.AutoCheckCore)
	_ = cfg.MustGet("updates", "auto_check_self").Update(s.AutoCheckSelf)
	_ = cfg.MustGet("updates", "default_interval_hours").Update(s.DefaultIntervalHours)

	_ = cfg.MustGet("core", "proxy", "enabled").Update(s.ProxyEnabled)
	_ = cfg.MustGet("core", "url_test_url").Update(s.URLTestURL)
	_ = cfg.MustGet("core", "log", "level").Update(s.CoreLogLevel)
	_ = cfg.MustGet("core", "traffic_graph_history").Update(s.TrafficGraphHistory)
	_ = cfg.MustGet("ui", "show_logs").Update(s.ShowLogs)

	_ = cfg.MustGet("updates", "auto_update_configs").Update(s.AutoUpdateConfigs)
	_ = cfg.MustGet("updates", "auto_update_configs_interval_hours").Update(s.AutoUpdateConfigsIntervalHours)
	_ = cfg.MustGet("updates", "auto_update_on_hash_mismatch").Update(s.AutoUpdateOnHashMismatch)
	_ = cfg.MustGet("updates", "auto_restart_on_config_update").Update(s.AutoRestartOnConfigUpdate)
	_ = cfg.MustGet("updates", "background_update_check_interval_hours").Update(s.BackgroundUpdateCheckIntervalHours)
	if err := cfg.Save(); err != nil {
		b.toastErr(err)
		return err
	}
	b.emit("settings:changed", s)
	b.emitTheme()
	b.toastT("success", []string{"settings", "saved"})
	return nil
}

// ResetData deletes config.yaml, profiles.yaml and the configs/ folder, then
// quits the application. The legacy Gio UI restarted the process afterwards;
// the Wails build has no restart mechanism, so it exits instead and the user
// relaunches manually.
func (b *Bindings) ResetData() error {
	dataDir := b.app.Controller.Config().DataDir
	paths := []string{
		filepath.Join(dataDir, "config.yaml"),
		filepath.Join(dataDir, "profiles.yaml"),
		filepath.Join(dataDir, "configs"),
	}
	var errs []string
	for _, p := range paths {
		if err := os.RemoveAll(p); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", p, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("reset data: %s", strings.Join(errs, "; "))
	}
	if b.ic != nil {
		b.ic.Close()
	}
	if wailsAppInstance != nil {
		wailsAppInstance.Quit()
	}
	return nil
}

// GetAppLogs returns the latest application log lines.
func (b *Bindings) GetAppLogs() []string {
	return b.app.Controller.GetLogLines()
}

// GetCoreLogs returns the latest sing-box core log lines.
func (b *Bindings) GetCoreLogs() []string {
	return b.app.Controller.GetCoreLogCleanLines()
}

// ClearAppLogs clears the application log buffer.
func (b *Bindings) ClearAppLogs() {
	b.app.Controller.ClearLogs()
}

// ClearCoreLogs clears the sing-box core log buffer.
func (b *Bindings) ClearCoreLogs() {
	b.app.Controller.ClearCoreLogs()
}

// emit sends an event to the frontend if a Wails application reference is available.
func (b *Bindings) emit(name string, data any) {
	if wailsAppInstance != nil {
		wailsAppInstance.Event.Emit(name, data)
	}
	if b.ws != nil {
		b.ws.broadcast(name, data)
	}
}

// notify emits a user-facing notification event with a localized title and body.
func (b *Bindings) notify(title, body string) {
	b.emit("notification", map[string]string{"title": title, "body": body})
}

// showDialog emits a modal dialog event for the frontend.
func (b *Bindings) showDialog(title, body string) {
	b.emit("dialog:show", map[string]string{"title": title, "body": body})
}

// emitConfigsChanged pushes the current profile list and active profile to the UI.
func (b *Bindings) emitConfigsChanged() {
	b.emit("configs:changed", map[string]any{
		"configs": b.app.Controller.GetConfigs(),
		"active":  b.GetActiveConfig(),
	})
}

// WailsApp is the Wails v3 binding layer for the sing-box-ez GUI.
type WailsApp struct {
	app    *apppkg.App
	assets embed.FS
}

// New creates a new Wails GUI instance.
func New(app *apppkg.App, assets embed.FS) *WailsApp {
	return &WailsApp{
		app:    app,
		assets: assets,
	}
}

// wailsAppInstance is used by Bindings to emit events. Set during Run.
var wailsAppInstance *application.App

// Run starts the Wails event loop.
func (w *WailsApp) Run() error {
	bindings := &Bindings{
		app:                w.app,
		ctx:                context.Background(),
		localeKeys:         make(map[string]struct{}),
		localeWildcards:    make(map[string]struct{}),
		pendingStyleChecks: make(map[string]func(string)),
	}

	// Create the interactive controller up front so the tray and the WebSocket
	// IPC server share the same start/stop flow as the window bindings.
	ic := core.NewInteractiveController(w.app.Controller)
	bindings.ic = ic

	wailsApp := application.New(application.Options{
		Name:        "sing-box-ez",
		Description: "sing-box-ez graphical interface",
		Linux: application.LinuxOptions{
			ProgramName: "sing-box-ez",
		},
		Services: []application.Service{
			application.NewService(bindings),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(w.assets),
		},
	})
	wailsAppInstance = wailsApp

	// Load embedded/user themes.
	cfg := w.app.Controller.Config()
	if themeColl, err := themes.Load(cfg.DataDir); err != nil {
		w.app.Logger.Root.Warnf("failed to load themes: %v", err)
	} else {
		bindings.theme = themeColl
	}

	// Build a standalone Wails bindings collection used by the WebSocket IPC
	// server. globalApplication must be set before Add() is called.
	wailsBindings := application.NewBindings(nil, nil)
	if err := wailsBindings.Add(application.NewService(bindings)); err != nil {
		return fmt.Errorf("register websocket bindings: %w", err)
	}
	bindings.ws = newWSServer(wailsBindings)
	if err := bindings.ws.start(); err != nil {
		return fmt.Errorf("start websocket ipc: %w", err)
	}

	win := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            localengine.T("app", "title"),
		Width:            1024,
		Height:           768,
		BackgroundColour: application.NewRGB(15, 23, 42),
		URL:              "/",
	})

	// Apply the configured theme background and react to OS theme changes.
	bindings.applyWindowBackground(win)
	wailsApp.Event.OnApplicationEvent(events.Common.ThemeChanged, func(_ *application.ApplicationEvent) {
		bindings.emitTheme()
		bindings.applyWindowBackground(win)
	})
	bindings.emitTheme()

	trayIcon := tray.New(
		w.app.Logger.Root,
		func() { win.Show() },
		func() { win.Hide() },
		func() { wailsApp.Quit() },
		func() bool { return w.app.Controller.IsRunning() },
		func() { go func() { _ = ic.StartService() }() },
		func() { go func() { _ = ic.StopService() }() },
	)
	_ = trayIcon.Start()

	ic.OnStatusChange = func(running bool) {
		bindings.emit("status:changed", Status{Running: running, PID: w.app.Controller.GetPID()})
		trayIcon.Refresh()
	}
	ic.OnLog = func(msg string) {
		bindings.emit("log:app", map[string]string{"line": msg})
	}
	ic.OnConfigUpdate = func() {
		bindings.emitConfigsChanged()
	}
	ic.OnVersionChange = func(ver string) {
		bindings.emit("core:version", map[string]string{"installed": ver})
	}
	ic.OnLatestVersion = func(ver string) {
		bindings.emit("core:version", map[string]string{"latest": ver})
	}
	ic.OnNotification = func(title, body string) {
		bindings.notify(title, body)
	}
	ic.OnAutoRestart = func() {
		bindings.notify(
			localengine.T("notify", "core_crashed", "title"),
			localengine.T("notify", "core_crashed", "body"),
		)
	}
	ic.OnUpdateCheckDue = func() {
		bindings.runUpdateChecks()
	}
	ic.OnCoreMissing = func() {
		title := localengine.T("app", "title")
		body := localengine.T("notify", "core_missing", "body")
		bindings.notify(title, body)
		bindings.showDialog(title, body)
	}
	ic.OnConfigMissing = func() {
		title := localengine.T("app", "title")
		body := localengine.T("notify", "config_missing", "body")
		bindings.notify(title, body)
		bindings.showDialog(title, body)
	}
	ic.OnConfigStyleCheck = func(style inboundstyle.Style, rec *config.ConfigRecord, choose func(string)) {
		bindings.styleCheckMu.Lock()
		bindings.pendingStyleChecks[rec.Name] = choose
		bindings.styleCheckMu.Unlock()
		bindings.emit("config:style_check", map[string]string{
			"config": rec.Name,
			"style":  string(style),
		})
	}
	ic.OnPhaseChange = func(phase string) {
		bindings.setPhaseHint(phase)
	}

	// Run the startup update checks (self-update, then core update), mirroring
	// the legacy Gio startup flow.
	go bindings.runUpdateChecks()

	// Subscribe to core log lines and forward them as events.
	if ic.Controller != nil {
		ic.Controller.LogProcessor().AddSubscriber(func(line string) {
			bindings.emit("log:core", map[string]string{"line": line})
		})
	}

	// Start the background core poller that emits live traffic events and
	// pushes api:state updates.
	bindings.startCorePoller()

	return wailsApp.Run()
}
