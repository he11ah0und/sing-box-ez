//go:build !nogui

package wails

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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
	ctx        context.Context
	app        *apppkg.App
	ws         *wsServer
	ic         *core.InteractiveController
	theme      *themes.Collection
	localeKeys map[string]struct{}
	// localeWildcards holds registered wildcard prefixes (without the ".*"
	// suffix); they expand to all matching leaf paths of the current language.
	localeWildcards map[string]struct{}
	localeReady     bool
	localeMu        sync.Mutex

	// core is the background core state poller holding the graph history and
	// the last known API state (phase, groups, connections).
	core *state.Poller

	// windowVisible tracks whether the main window is currently shown.
	// High-frequency telemetry events (gatedWhileHidden) are dropped while it
	// is hidden so the backend does not feed a webview nobody is looking at.
	windowVisible atomic.Bool

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

	// updateMu guards updateCancels.
	updateMu sync.Mutex
	// updateCancels holds the cancel funcs of in-flight downloads, keyed by
	// updater target ("app" / "core"); CancelUpdate consumes them.
	updateCancels map[string]context.CancelFunc
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
	lastUpdate := rec.LastUpdate.Time
	if rec.IsLocal() {
		// A local profile has no download history; its file mtime is the
		// meaningful "last updated" moment.
		if mt, err := b.app.Controller.LocalConfigModTime(rec.Name); err == nil {
			lastUpdate = mt
		}
	}
	return &ActiveConfig{
		ConfigRecord:  rec,
		LastUpdateAgo: version.HumanDuration(lastUpdate),
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

// AddConfig creates a new profile. For a local profile, a non-empty
// sourcePath imports that file as the profile's config content.
func (b *Bindings) AddConfig(rec config.ConfigRecord, sourcePath string) error {
	if err := b.app.Controller.AddConfig(rec); err != nil {
		b.toastErr(err)
		return err
	}
	if sourcePath != "" && rec.IsLocal() {
		if err := b.app.Controller.ImportLocalConfigFile(rec.Name, sourcePath); err != nil {
			b.toastErr(err)
			return err
		}
	}
	return nil
}

// EditConfig updates an existing profile. For a local profile, a non-empty
// sourcePath replaces the cached config content with that file.
func (b *Bindings) EditConfig(oldName string, rec config.ConfigRecord, sourcePath string) error {
	if err := b.app.Controller.EditConfig(oldName, rec); err != nil {
		b.toastErr(err)
		return err
	}
	if sourcePath != "" && rec.IsLocal() {
		if err := b.app.Controller.ImportLocalConfigFile(rec.Name, sourcePath); err != nil {
			b.toastErr(err)
			return err
		}
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
		// A user-initiated cancel is not an error: the phase poller lands on
		// stopped by itself once the core stays down.
		if errors.Is(err, core.ErrStartCancelled) {
			return nil
		}
		// Missing privileges for a tun/transparent config: the frontend shows
		// a dialog that leads to the privilege settings instead of a toast.
		if errors.Is(err, core.ErrPrivilegesRequired) {
			b.emit("privileges:required", struct{}{})
			return nil
		}
		b.toastErr(err, "main", "btn", "start")
	}
	return err
}

// CancelStart aborts an in-flight start flow (config download, style check,
// process spawn) and stops the core if it already came up — this is the
// cancel button of the start/waiting phases.
func (b *Bindings) CancelStart() {
	if b.ic != nil {
		b.ic.CancelStart()
		return
	}
	_ = b.app.Controller.Stop()
}

// SkipConfigUpdate aborts only the in-flight config download of the start
// flow (the "skip" button of the preparing_config phase); the start
// continues with the cached config.
func (b *Bindings) SkipConfigUpdate() {
	if b.app.Controller != nil {
		b.app.Controller.SkipConfigDownload()
	}
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

// Restart restarts the sing-box core. A success toast is shown because the
// phase flash (connected → restarting → connected) can be shorter than a
// poll tick and otherwise leaves no visible trace of the restart.
func (b *Bindings) Restart() error {
	b.setPhaseHint(state.PhaseRestarting)
	defer b.setPhaseHint("")
	if err := b.app.Controller.Restart(); err != nil {
		b.toastErr(err, "main", "btn", "restart")
		return err
	}
	b.toastT("success", []string{"main", "btn", "restarted"})
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

// registerUpdateCancel stores the cancel func of an in-flight download.
func (b *Bindings) registerUpdateCancel(target string, cancel context.CancelFunc) {
	b.updateMu.Lock()
	b.updateCancels[target] = cancel
	b.updateMu.Unlock()
}

// unregisterUpdateCancel drops the cancel func of a finished download.
func (b *Bindings) unregisterUpdateCancel(target string) {
	b.updateMu.Lock()
	delete(b.updateCancels, target)
	b.updateMu.Unlock()
}

// CancelUpdate cancels an in-flight app/core download; the download binding
// returns nil on cancellation (no error toast for a user-requested stop).
func (b *Bindings) CancelUpdate(target string) {
	b.updateMu.Lock()
	cancel := b.updateCancels[target]
	b.updateMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// DownloadCore downloads and installs the latest sing-box core.
func (b *Bindings) DownloadCore() error {
	ctx, cancel := context.WithCancel(b.ctx)
	defer cancel()
	b.registerUpdateCancel("core", cancel)
	defer b.unregisterUpdateCancel("core")
	_, err := b.app.Controller.DownloadCoreContext(ctx, func(downloaded, total int64) {
		progress := 0
		if total > 0 {
			progress = int(float64(downloaded) / float64(total) * 100)
		}
		b.emit("update:progress", map[string]int64{
			"downloaded": downloaded,
			"total":      total,
			"progress":   int64(progress),
		})
	}, func() {
		b.emit("update:phase", "installing")
	})
	if err != nil && (errors.Is(err, context.Canceled) || ctx.Err() != nil) {
		// User-requested cancel (CancelUpdate): not an error, no toast.
		return nil
	}
	if err != nil {
		b.toastErr(err)
		return err
	}
	b.toastT("success", []string{"core", "update", "installed"})
	// Replacing the core binary drops its file capabilities; open settings
	// views re-read the setcap state on this event.
	b.emit("privileges:changed", struct{}{})
	return nil
}

// IsCoreManaged reports whether the core binary is managed by the app
// (official source mode) — the core update UI is hidden otherwise.
func (b *Bindings) IsCoreManaged() bool {
	return b.app.Controller.IsCoreManaged()
}

// ListCoreVersions lists the installable core releases (newest first,
// prereleases included) for the official-mode version picker.
func (b *Bindings) ListCoreVersions() []core.CoreRelease {
	ctx, cancel := context.WithTimeout(b.ctx, 30*time.Second)
	defer cancel()
	releases, err := b.app.Controller.ListCoreVersions(ctx)
	if err != nil {
		b.toastErr(err)
		return nil
	}
	return releases
}

// DownloadCoreVersion downloads and installs the picked core release.
func (b *Bindings) DownloadCoreVersion(ver string) error {
	ctx, cancel := context.WithCancel(b.ctx)
	defer cancel()
	b.registerUpdateCancel("core", cancel)
	defer b.unregisterUpdateCancel("core")
	_, err := b.app.Controller.DownloadCoreVersionContext(ctx, ver, func(downloaded, total int64) {
		progress := 0
		if total > 0 {
			progress = int(float64(downloaded) / float64(total) * 100)
		}
		b.emit("update:progress", map[string]int64{
			"downloaded": downloaded,
			"total":      total,
			"progress":   int64(progress),
		})
	}, func() {
		b.emit("update:phase", "installing")
	})
	if err != nil && (errors.Is(err, context.Canceled) || ctx.Err() != nil) {
		// User-requested cancel (CancelUpdate): not an error, no toast.
		return nil
	}
	if err != nil {
		b.toastErr(err)
		return err
	}
	b.toastT("success", []string{"core", "update", "installed"})
	b.emit("privileges:changed", struct{}{})
	return nil
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

// SetTheme changes the active theme, persists it to config and notifies the UI.
func (b *Bindings) SetTheme(name string) ThemePayload {
	_ = b.app.Controller.Config().MustGet("ui", "theme").Update(name)
	_ = b.app.Controller.Config().Save()
	t := b.themePayload()
	b.emit("theme:changed", t)
	return t
}

// GetConnectionsSort returns the persisted connections-tab sort mode
// ("date" | "traffic" | "total"). The entry has no settings-UI control (the
// selector lives on the main page), so it is not part of GetConfigValues.
func (b *Bindings) GetConnectionsSort() string {
	return b.app.Controller.Config().MustGet("ui", "connections_sort").String()
}

// SetConnectionsSort persists the connections-tab sort mode picked in the
// main page selector. Unknown modes are rejected.
func (b *Bindings) SetConnectionsSort(mode string) error {
	switch mode {
	case "date", "traffic", "total":
	default:
		return fmt.Errorf("unknown connections sort mode %q", mode)
	}
	if err := b.app.Controller.Config().MustGet("ui", "connections_sort").Update(mode); err != nil {
		return err
	}
	if err := b.app.Controller.Config().Save(); err != nil {
		b.toastErr(err)
		return err
	}
	return nil
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
					b.app.Logger.Root.TWarnf("gui.locale_wildcard_no_match", k)
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

// QuitApp terminates the application. Used by the blocking channel-error
// dialog shown when an externally managed build cannot verify its own
// version.
func (b *Bindings) QuitApp() {
	if wailsAppInstance != nil {
		wailsAppInstance.Quit()
	}
}

// GetAppLogs returns the latest application log lines.
func (b *Bindings) GetAppLogs() []string {
	return b.app.Controller.GetLogLines()
}

// GetCoreLogs returns the latest sing-box core log lines. Raw lines (with
// ANSI escapes) are returned so the debug page can render them with the
// same colors as the live log:core event stream; plain-text consumers
// (clipboard copy) use GetCoreLogCleanLines.
func (b *Bindings) GetCoreLogs() []string {
	return b.app.Controller.GetCoreLogLines()
}

// ClearAppLogs clears the application log buffer.
func (b *Bindings) ClearAppLogs() {
	b.app.Controller.ClearLogs()
}

// ClearCoreLogs clears the sing-box core log buffer.
func (b *Bindings) ClearCoreLogs() {
	b.app.Controller.ClearCoreLogs()
}

// gatedWhileHidden lists the high-frequency telemetry events dropped while the
// main window is hidden (per-second state pushes and per-line log events).
// Low-frequency state changes and user-facing events always pass through.
var gatedWhileHidden = map[string]bool{
	"api:state":       true,
	"traffic:updated": true,
	"log:app":         true,
	"log:core":        true,
}

// emit sends an event to the frontend if a Wails application reference is available.
func (b *Bindings) emit(name string, data any) {
	if wailsAppInstance != nil && (b.windowVisible.Load() || !gatedWhileHidden[name]) {
		wailsAppInstance.Event.Emit(name, data)
	}
	if b.ws != nil {
		b.ws.broadcast(name, data)
	}
}

// setWindowVisible updates the visibility gate. When the window reappears a
// fresh API snapshot is pushed so the UI catches up immediately instead of
// waiting for the poller's next tick.
func (b *Bindings) setWindowVisible(visible bool) {
	if b.windowVisible.Swap(visible) == visible {
		return
	}
	if visible && b.core != nil {
		b.emit("api:state", b.core.State())
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
		updateCancels:      make(map[string]context.CancelFunc),
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
		w.app.Logger.Root.TWarnf("themes.load_failed", err)
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

	// Track window visibility: the gated telemetry events (see emit) are
	// dropped while the window is minimised or hidden to the tray, so an
	// unseen webview is not fed per-second snapshots and log lines.
	bindings.windowVisible.Store(true)
	win.OnWindowEvent(events.Common.WindowMinimise, func(_ *application.WindowEvent) {
		bindings.setWindowVisible(false)
	})
	win.OnWindowEvent(events.Common.WindowUnMinimise, func(_ *application.WindowEvent) {
		bindings.setWindowVisible(true)
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
		func() {
			win.Show()
			bindings.setWindowVisible(true)
		},
		func() {
			win.Hide()
			bindings.setWindowVisible(false)
		},
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
	ic.OnConfigsUpdateFailed = func(failures []core.ConfigUpdateFailure) {
		bindings.emit("configs:update_failed", failures)
	}
	// The start flow fell back to the cached config: the frontend offers a
	// retry through the running core's proxy (config:download_failed).
	if w.app.Controller != nil {
		w.app.Controller.OnConfigDownloadFailed = func(name string, err error) {
			bindings.emit("config:download_failed", map[string]string{
				"name": name,
				"kind": core.DownloadErrorKind(err),
			})
		}
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
	ic.OnCoreIncompatible = func() {
		bindings.emit("core:incompatible", map[string]string{"min": core.MinCoreVersion})
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
