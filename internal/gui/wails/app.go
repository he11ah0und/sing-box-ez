//go:build !nogui

package wails

import (
	"context"
	"embed"
	"fmt"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	apppkg "sing-box-ez/internal/app"
	"sing-box-ez/internal/app/themes"
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
	"sing-box-ez/internal/core/api"
	"sing-box-ez/internal/framework/localengine"
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
	localeReady        bool
	localeMissingWarns map[string]struct{}
	localeMu           sync.Mutex
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

// Greet is a temporary binding used to verify the Wails bridge.
func (b *Bindings) Greet(name string) string {
	return fmt.Sprintf("Hello %s, Wails v3 is working!", name)
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
func (b *Bindings) GetActiveConfig() *config.ConfigRecord {
	return b.app.Controller.GetActiveConfig()
}

// ActivateConfig sets the active profile by name.
func (b *Bindings) ActivateConfig(name string) error {
	return b.app.Controller.ActivateConfig(name)
}

// AddConfig creates a new profile.
func (b *Bindings) AddConfig(rec config.ConfigRecord) error {
	return b.app.Controller.AddConfig(rec)
}

// EditConfig updates an existing profile.
func (b *Bindings) EditConfig(oldName string, rec config.ConfigRecord) error {
	return b.app.Controller.EditConfig(oldName, rec)
}

// DeleteConfig removes a profile by name.
func (b *Bindings) DeleteConfig(name string) error {
	return b.app.Controller.DeleteConfig(name)
}

// Start starts the sing-box core with the active config.
func (b *Bindings) Start() error {
	return b.app.Controller.Start()
}

// Stop stops the sing-box core.
func (b *Bindings) Stop() error {
	return b.app.Controller.Stop()
}

// Restart restarts the sing-box core.
func (b *Bindings) Restart() error {
	return b.app.Controller.Restart()
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
func (b *Bindings) RegisterLocaleKeys(keys []string) map[string]string {
	b.localeMu.Lock()
	defer b.localeMu.Unlock()
	for _, k := range keys {
		b.localeKeys[k] = struct{}{}
	}
	return b.localeKeyValuesLocked()
}

// LocaleReady tells the backend that the initial set of UI keys is registered.
// It warns about missing/object keys and refreshes registered values.
func (b *Bindings) LocaleReady() {
	b.localeMu.Lock()
	b.localeReady = true
	b.warnLocaleKeysLocked()
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
	for k := range b.localeKeys {
		if v, ok := localengine.LookupString(localengine.CurrentLanguage(), strings.Split(k, ".")...); ok {
			values[k] = v
		}
	}
	return values
}

func (b *Bindings) warnLocaleKeysLocked() {
	lang := localengine.CurrentLanguage()
	// Warn once per missing/object key.
	for k := range b.localeKeys {
		if _, warned := b.localeMissingWarns[k]; warned {
			continue
		}
		path := strings.Split(k, ".")
		v, ok := localengine.Lookup(lang, path...)
		if !ok {
			// Try English fallback before warning.
			v, ok = localengine.Lookup("en", path...)
		}
		if !ok {
			b.app.Logger.Root.Warnf("locale key %q not found", k)
			b.localeMissingWarns[k] = struct{}{}
			continue
		}
		if _, isString := v.(string); !isString {
			b.app.Logger.Root.Warnf("locale key %q resolves to an object, not a string", k)
			b.localeMissingWarns[k] = struct{}{}
		}
	}
	// Optional: warn about unused leaf paths in the current locale.
	// Disabled by default because the bundled locales contain many keys from the
	// legacy Gio UI that are not used by the Wails frontend yet.
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
	if err := cfg.Save(); err != nil {
		return err
	}
	b.emit("settings:changed", s)
	b.emitTheme()
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

// GetTraffic returns a one-shot snapshot of core traffic and connections.
func (b *Bindings) GetTraffic() api.Status {
	client := b.app.Controller.APIClient()
	if client == nil {
		return api.Status{}
	}
	status, err := client.Status(b.ctx)
	if err != nil {
		return api.Status{}
	}
	return *status
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
		localeMissingWarns: make(map[string]struct{}),
	}

	wailsApp := application.New(application.Options{
		Name:        "sing-box-ez",
		Description: "sing-box-ez graphical interface",
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
		Title:            "sing-box-ez",
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
		func() { _ = w.app.Controller.Start() },
		func() { _ = w.app.Controller.Stop() },
	)
	_ = trayIcon.Start()

	ic := core.NewInteractiveController(w.app.Controller)
	bindings.ic = ic
	ic.OnStatusChange = func(running bool) {
		bindings.emit("status:changed", Status{Running: running, PID: w.app.Controller.GetPID()})
		trayIcon.Refresh()
	}
	ic.OnLog = func(msg string) {
		bindings.emit("log:app", map[string]string{"line": msg})
	}
	ic.OnConfigUpdate = func() {
		bindings.emit("configs:changed", map[string]any{
			"configs": w.app.Controller.GetConfigs(),
			"active":  w.app.Controller.GetActiveConfig(),
		})
	}
	ic.OnVersionChange = func(ver string) {
		bindings.emit("core:version", map[string]string{"installed": ver})
	}
	ic.OnLatestVersion = func(ver string) {
		bindings.emit("core:version", map[string]string{"latest": ver})
	}
	ic.OnNotification = func(title, body string) {
		bindings.emit("notification", map[string]string{"title": title, "body": body})
	}

	// Subscribe to core log lines and forward them as events.
	if ic.Controller != nil {
		ic.Controller.LogProcessor().AddSubscriber(func(line string) {
			bindings.emit("log:core", map[string]string{"line": line})
		})
	}

	// Start the background traffic poller that emits live traffic events.
	bindings.startTrafficPoller()

	return wailsApp.Run()
}
