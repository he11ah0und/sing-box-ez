package core

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/i18n"
	"sing-box-ez/internal/updater"
	"sing-box-ez/internal/util/paths"
	"sing-box-ez/internal/version"
)

// InteractiveController extends Controller with callbacks, background loops,
// and interactive features for GUI/TUI frontends.
type InteractiveController struct {
	*Controller

	// UI callbacks (optional, invoked when state changes)
	OnStatusChange    func(running bool)
	OnLog             func(msg string)
	OnConfigUpdate    func()
	OnVersionChange   func(ver string)
	OnPrivilegeChange func(active bool)
	OnLatestVersion   func(ver string)
	OnNotification    func(title, body string)
	OnAutoRestart     func()
}

// NewInteractiveController creates an interactive controller, initializes i18n,
// starts background maintenance loops, and wires callbacks.
func NewInteractiveController(cfg *config.AppConfig) *InteractiveController {
	// Initialize i18n
	if !cfg.GetFirstRunDone() {
		lang := i18n.DetectSystemLanguage()
		cfg.SetLanguage(lang)
		_ = cfg.Save()
		i18n.SetLanguage(lang)
	} else if lang := cfg.GetLanguage(); lang != "" {
		i18n.SetLanguage(lang)
	}

	ctrl := NewController(cfg)
	ic := &InteractiveController{Controller: ctrl}

	ctrl.Logger().OnAutoRestart = func() {
		if ic.OnAutoRestart != nil {
			ic.OnAutoRestart()
		}
	}

	// Background loops
	go ic.updateChecker()
	go ic.statusChecker()

	return ic
}

// Log logs a message and optionally invokes the OnLog callback.
func (ic *InteractiveController) Log(msg string) {
	ic.Controller.Log(msg)
	if ic.OnLog != nil {
		ic.OnLog(msg)
	}
}

// ---------- Privileges ----------

func (ic *InteractiveController) HasRequiredPrivileges() bool {
	switch runtime.GOOS {
	case "linux":
		if HasNetAdminCapability(GetCorePath()) {
			return true
		}
		return ic.Config().RunAsAdmin
	case "windows":
		return IsAdmin()
	default:
		return true
	}
}

func (ic *InteractiveController) RefreshPrivilegeStatus() string {
	if runtime.GOOS != "linux" {
		return ""
	}
	if HasNetAdminCapability(GetCorePath()) {
		return "active"
	}
	return "root_required"
}

// ---------- Version & download ----------

func (ic *InteractiveController) GetInstalledCoreVersion() (string, error) {
	return GetCoreVersion(GetCorePath())
}

func (ic *InteractiveController) GetLatestCoreVersion() (string, error) {
	return GetLatestVersion()
}

func (ic *InteractiveController) DownloadCoreWithProgress(onProgress func(downloaded, total int64)) (string, error) {
	ver, err := GetLatestVersion()
	if err != nil {
		return "", err
	}
	ic.Log("Latest version: v" + ver)

	path, err := DownloadCore("", onProgress)
	if err != nil {
		return "", err
	}
	ic.Log("Core downloaded to: " + path)
	return path, nil
}

// ---------- Self update ----------

func (ic *InteractiveController) CheckSelfUpdate() (*updater.UpdateInfo, error) {
	return updater.CheckUpdate(version.Branch)
}

// ---------- Update checking ----------

// PrivilegeAction describes a single action available in the privilege dialog/tab.
type PrivilegeAction struct {
	ID      string
	Label   string
	Handler func() error
}

// PrivilegeDialog describes the platform-specific privilege elevation dialog.
type PrivilegeDialog struct {
	Title   string
	Message string
	Actions []PrivilegeAction
}

// GetPrivilegeDialog returns the dialog definition for the current platform,
// or nil if no privilege dialog is needed (e.g. macOS).
func (ic *InteractiveController) GetPrivilegeDialog() *PrivilegeDialog {
	switch runtime.GOOS {
	case "darwin":
		return nil
	case "windows":
		return &PrivilegeDialog{
			Title:   i18n.T("dialog.privileges.title"),
			Message: i18n.T("dialog.privileges.msg_windows"),
			Actions: []PrivilegeAction{
				{
					ID:    "restart_admin",
					Label: i18n.T("dialog.privileges.btn_restart_admin"),
					Handler: func() error {
						return ic.RestartAsAdmin()
					},
				},
			},
		}
	case "linux":
		return &PrivilegeDialog{
			Title:   i18n.T("dialog.privileges.title"),
			Message: i18n.T("dialog.privileges.msg_linux"),
			Actions: []PrivilegeAction{
				{
					ID:    "setcap",
					Label: i18n.T("dialog.privileges.btn_setcap"),
					Handler: func() error {
						return ic.ApplySetcap()
					},
				},
				{
					ID:    "run_as_admin",
					Label: i18n.T("dialog.privileges.btn_run_as_admin"),
					Handler: func() error {
						ic.cfg.SetRunAsAdmin(true)
						ic.SetElevated(true)
						return ic.cfg.Save()
					},
				},
			},
		}
	default:
		return nil
	}
}

// PrivilegeTabState describes the platform-specific state for the Core privileges tab.
type PrivilegeTabState struct {
	Mode                string // "windows", "linux", "macos"
	IsAdmin             bool
	HasSetcap           bool
	RunAsAdmin          bool
	AdminStatusText     string
	AdminStatusColor    string // "green", "yellow"
	AdminLabel          string
	PrivilegeText       string
	PrivilegeColor      string // "green", "yellow"
	ShowRestartAdminBtn bool
	ShowSetcapBtn       bool
}

// GetPrivilegeTabState returns the current privilege state for rendering the Core tab.
func (ic *InteractiveController) GetPrivilegeTabState() PrivilegeTabState {
	state := PrivilegeTabState{
		Mode:       runtime.GOOS,
		RunAsAdmin: ic.cfg.RunAsAdmin,
	}

	switch runtime.GOOS {
	case "windows":
		state.IsAdmin = IsAdmin()
		if state.IsAdmin {
			state.AdminStatusText = i18n.T("core.privileges.admin")
			state.AdminStatusColor = "green"
		} else {
			state.AdminStatusText = i18n.T("core.privileges.user")
			state.AdminStatusColor = "yellow"
		}
		state.ShowRestartAdminBtn = !state.IsAdmin
	case "linux":
		state.HasSetcap = HasNetAdminCapability(GetCorePath())
		if state.HasSetcap {
			state.AdminLabel = i18n.T("core.admin.label_root_setcap")
		} else {
			state.AdminLabel = i18n.T("core.admin.label_root_pkexec")
		}
		state.ShowSetcapBtn = true
		status := ic.RefreshPrivilegeStatus()
		switch status {
		case "active":
			state.PrivilegeText = i18n.T("core.privileges.setcap_active")
			state.PrivilegeColor = "green"
		case "root_required":
			state.PrivilegeText = i18n.T("core.privileges.root_required")
			state.PrivilegeColor = "yellow"
		}
	default:
		state.AdminLabel = i18n.T("core.admin.label")
	}

	return state
}

func (ic *InteractiveController) CoreExists() bool {
	return CoreExists()
}

func (ic *InteractiveController) IsAdmin() bool {
	return IsAdmin()
}

func (ic *InteractiveController) HasNetAdminCapability() bool {
	return HasNetAdminCapability(GetCorePath())
}

func (ic *InteractiveController) ApplySetcap() error {
	return SetNetAdminCapabilityGUI(GetCorePath())
}

func (ic *InteractiveController) ApplySelfUpdate(assetURL string, onProgress func(int64, int64)) error {
	return updater.ApplyUpdate(assetURL, onProgress)
}

// HasCachedConfig reports whether a cached config exists for the given name.
func (ic *InteractiveController) HasCachedConfig(name string) bool {
	return HasCachedConfig(name)
}

// DownloadConfigFor downloads a config by URL and caches it under the given name.
func (ic *InteractiveController) DownloadConfigFor(name, url string) error {
	return DownloadConfigFor(name, url)
}

// StartCore starts the core and logs the result.
func (ic *InteractiveController) StartCore() error {
	_, err := ic.PrepareConfig()
	if err != nil {
		ic.Log(err.Error())
		return err
	}
	if err := ic.Start(); err != nil {
		ic.Log("Failed to start: " + err.Error())
		return err
	}
	ic.Log("Core started")
	return nil
}

// StopCore stops the core and logs the result.
func (ic *InteractiveController) StopCore() error {
	if err := ic.Stop(); err != nil {
		ic.Log("Failed to stop: " + err.Error())
		return err
	}
	ic.Log("Core stopped")
	return nil
}

// RestartCore restarts the core and logs the result.
func (ic *InteractiveController) RestartCore() error {
	if err := ic.Restart(); err != nil {
		ic.Log("Failed to restart: " + err.Error())
		return err
	}
	ic.Log("Core restarted")
	return nil
}

// Logf is a convenience wrapper for formatted logging.
func (ic *InteractiveController) Logf(format string, args ...interface{}) {
	ic.Log(fmt.Sprintf(format, args...))
}

func (ic *InteractiveController) CheckUpdates() (*updater.UpdateInfo, string, string, error) {
	info, err := updater.CheckUpdate(version.Branch)
	if err == nil && info.ReleaseCount > 0 {
		return info, "", "", nil
	}
	currentVer, err := GetCoreVersion(GetCorePath())
	if err != nil || currentVer == "" {
		return nil, "", "", err
	}
	latestVer, err := GetLatestVersion()
	if err != nil {
		return nil, currentVer, "", err
	}
	return nil, currentVer, latestVer, nil
}

// ---------- Background loops ----------

func (ic *InteractiveController) updateChecker() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		if ic.isStopped() {
			return
		}
		active := ic.Config().GetActiveConfig()
		if active != nil && active.ShouldUpdate() && ic.IsRunning() {
			ic.Log("Auto-updating config...")
			ic.SetConfigName(active.Name)
			if err := ic.UpdateConfig(); err != nil {
				ic.Log("Auto-update failed: " + err.Error())
			} else {
				ic.Config().SetLastUpdateFor(active.Name, time.Now())
				_ = ic.Config().Save()
				if ic.OnConfigUpdate != nil {
					ic.OnConfigUpdate()
				}
				ic.Log("Config auto-updated, restarting core...")
				if err := ic.Restart(); err != nil {
					ic.Log("Auto-restart failed: " + err.Error())
				}
			}
		}
	}
}

func (ic *InteractiveController) statusChecker() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	var lastRunning bool
	for range ticker.C {
		if ic.isStopped() {
			return
		}
		running := ic.IsRunning()
		if running != lastRunning {
			lastRunning = running
			if ic.OnStatusChange != nil {
				ic.OnStatusChange(running)
			}
		}
	}
}

// RestartAsAdmin restarts the application with elevated privileges (Windows only).
func (ic *InteractiveController) RestartAsAdmin() error {
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
	// #nosec G204 — powershell is a system binary; exe comes from os.Executable() and cwd from os.Getwd().
	cmd := exec.Command("powershell", "-WindowStyle", "hidden", "-Command",
		"Start-Process", "-FilePath", exe, "-Verb", "runAs", "-WorkingDirectory", cwd)
	return cmd.Start()
}

// GetLatestCoreVersionWithLog fetches the latest core version and logs errors.
func (ic *InteractiveController) GetLatestCoreVersionWithLog() (string, error) {
	ver, err := ic.GetLatestCoreVersion()
	if err != nil {
		ic.Log("Check failed: " + err.Error())
		return "", err
	}
	return ver, nil
}

// RestartAsAdminWithLog attempts to restart as admin and logs the result.
func (ic *InteractiveController) RestartAsAdminWithLog() error {
	err := ic.RestartAsAdmin()
	if err != nil {
		ic.Log("Failed to restart as admin: " + err.Error())
	}
	return err
}

// SetRunAsAdminWithLog updates the run-as-admin setting and logs the result.
func (ic *InteractiveController) SetRunAsAdminWithLog(checked bool) error {
	ic.cfg.SetRunAsAdmin(checked)
	ic.SetElevated(checked)
	if err := ic.cfg.Save(); err != nil {
		ic.Log("Failed to save admin setting: " + err.Error())
		return err
	}
	ic.Logf("Admin mode: %v", checked)
	return nil
}

// ApplySetcapWithLog applies setcap and logs the result.
func (ic *InteractiveController) ApplySetcapWithLog() error {
	err := ic.ApplySetcap()
	if err != nil {
		ic.Log("setcap failed: " + err.Error())
		ic.Log("Tip: run manually: sudo setcap cap_net_admin=+ep ./sing-box")
		return err
	}
	ic.Log("setcap applied successfully.")
	return nil
}

// DownloadCoreWithProgressWithLog downloads the core and logs the result.
func (ic *InteractiveController) DownloadCoreWithProgressWithLog(onProgress func(int64, int64)) (string, error) {
	path, err := ic.DownloadCoreWithProgress(onProgress)
	if err != nil {
		ic.Log("Failed to download core: " + err.Error())
		return "", err
	}
	return path, nil
}

// ApplySelfUpdateWithLog performs a self-update and logs the result.
func (ic *InteractiveController) ApplySelfUpdateWithLog(assetURL string, onProgress func(int64, int64)) error {
	if assetURL == "" {
		ic.Log("Self-update: no matching asset for this system")
		return fmt.Errorf("no matching asset")
	}
	err := ic.ApplySelfUpdate(assetURL, onProgress)
	if err != nil {
		ic.Log("Self-update failed: " + err.Error())
		return err
	}
	return nil
}

// AddFirstConfigWithLog adds the initial config during first-run and logs the result.
func (ic *InteractiveController) AddFirstConfigWithLog(name, url string) error {
	if url == "" {
		ic.Log("First run: empty config URL")
		return fmt.Errorf("empty config URL")
	}
	rec := config.ConfigRecord{
		Name:                name,
		URL:                 url,
		UpdateIntervalHours: ic.cfg.UpdateIntervalHours,
		Parent:              "user",
	}
	ic.cfg.AddConfig(rec)
	ic.cfg.SetActiveName(name)
	ic.cfg.SetFirstRunDone(true)
	_ = ic.cfg.Save()
	ic.SetConfigURL(url)
	ic.SetConfigName(name)
	ic.Log("First config added: " + name)
	return nil
}

// CheckSelfUpdateWithLog checks for self updates and logs the result.
func (ic *InteractiveController) CheckSelfUpdateWithLog() (*updater.UpdateInfo, error) {
	info, err := ic.CheckSelfUpdate()
	if err != nil {
		ic.Log("Update check failed: " + err.Error())
		return nil, err
	}
	if info.ReleaseCount == 0 {
		ic.Log("Already on latest version: " + info.Current)
		return nil, nil
	}
	ic.Logf("Update available: %s → %s (%d releases behind)", info.Current, info.Latest, info.ReleaseCount)
	return info, nil
}

// GetBranches fetches available repository branches.
func (ic *InteractiveController) GetBranches() ([]updater.Branch, error) {
	return updater.GetBranches()
}

// CheckSelfUpdateForBranch checks for updates on the specified branch and logs the result.
func (ic *InteractiveController) CheckSelfUpdateForBranch(branch string) (*updater.UpdateInfo, error) {
	info, err := updater.CheckUpdateForBranch(branch)
	if err != nil {
		ic.Log("Update check failed for branch " + branch + ": " + err.Error())
		return nil, err
	}
	if info.ReleaseCount == 0 {
		ic.Log("Branch " + branch + " is up to date: " + info.Current)
	} else {
		ic.Logf("Update available on %s: %s → %s", branch, info.Current, info.Latest)
	}
	return info, nil
}

// OpenDataFolderWithLog opens the data directory and logs errors.
func (ic *InteractiveController) OpenDataFolderWithLog() error {
	err := paths.OpenDataDir()
	if err != nil {
		ic.Log("Failed to open data folder: " + err.Error())
		return err
	}
	return nil
}

// FetchReleaseNotesWithLog fetches release notes and logs errors.
// Returns the release and an error (err != nil only when TagName is non-empty).
func (ic *InteractiveController) FetchReleaseNotesWithLog(commit string) (updater.Release, error) {
	release, err := updater.GetReleaseByTag(commit)
	if err != nil {
		if release.TagName == "" {
			return release, nil
		}
		ic.Log("Failed to fetch release notes: " + err.Error())
		return release, err
	}
	return release, nil
}

// SetLogLimitWithLog updates the log limit and logs the change.
func (ic *InteractiveController) SetLogLimitWithLog(v int) {
	ic.cfg.SetLogLimit(v)
	_ = ic.cfg.Save()
	ic.Logf("Log limit set to %d", v)
}

// SetDefaultIntervalWithLog updates the default update interval and logs the change.
func (ic *InteractiveController) SetDefaultIntervalWithLog(h int) {
	ic.cfg.SetDefaultUpdateInterval(h)
	_ = ic.cfg.Save()
	ic.Logf("Default interval set to %dh", h)
}

// UpdateConfigNowWithLog downloads a config and logs the result.
func (ic *InteractiveController) UpdateConfigNowWithLog(name, url string) error {
	err := ic.DownloadConfigFor(name, url)
	if err != nil {
		ic.Log("Update failed: " + err.Error())
		return err
	}
	ic.cfg.SetLastUpdateFor(name, time.Now())
	_ = ic.cfg.Save()
	ic.Log("Config updated: " + name)
	return nil
}

// AddConfigWithLog adds a config and logs the result.
func (ic *InteractiveController) AddConfigWithLog(rec config.ConfigRecord) error {
	if rec.Name == "" || rec.URL == "" {
		ic.Log("Name and URL are required")
		return fmt.Errorf("name and URL are required")
	}
	if ic.cfg.GetConfigByName(rec.Name) != nil {
		ic.Log("Config with this name already exists")
		return fmt.Errorf("config already exists")
	}
	ic.cfg.AddConfig(rec)
	if ic.cfg.GetActiveName() == "" {
		ic.cfg.SetActiveName(rec.Name)
		ic.SetConfigURL(rec.URL)
		ic.SetConfigName(rec.Name)
	}
	_ = ic.cfg.Save()
	ic.Log("Config added: " + rec.Name)
	return nil
}

// EditConfigWithLog edits or renames a config and logs the result.
func (ic *InteractiveController) EditConfigWithLog(oldName string, rec config.ConfigRecord) error {
	if rec.Name == "" || rec.URL == "" {
		ic.Log("Name and URL are required")
		return fmt.Errorf("name and URL are required")
	}
	if rec.Name != oldName {
		if ic.cfg.GetConfigByName(rec.Name) != nil {
			ic.Logf("Config with name %q already exists", rec.Name)
			return fmt.Errorf("config already exists")
		}
		ic.cfg.RenameConfig(oldName, rec.Name)
	}
	ic.cfg.UpdateConfig(rec.Name, rec)
	_ = ic.cfg.Save()
	if oldName == ic.cfg.GetActiveName() || rec.Name == ic.cfg.GetActiveName() {
		ic.SetConfigURL(rec.URL)
	}
	if rec.Name != oldName {
		ic.Log("Config renamed: " + oldName + " -> " + rec.Name)
	} else {
		ic.Log("Config updated: " + rec.Name)
	}
	return nil
}

// DeleteConfigWithLog deletes a config and logs the result.
func (ic *InteractiveController) DeleteConfigWithLog(name string) error {
	ic.cfg.RemoveConfig(name)
	_ = ic.cfg.Save()
	ic.Log("Config deleted: " + name)
	return nil
}

// ActivateConfigWithLog activates a cached config and logs the result.
func (ic *InteractiveController) ActivateConfigWithLog(name string) error {
	rec := ic.cfg.GetConfigByName(name)
	if rec == nil {
		return fmt.Errorf("config not found")
	}
	if !ic.HasCachedConfig(name) {
		ic.Log("No cached config for: " + name)
		return fmt.Errorf("no cached config")
	}
	ic.cfg.SetActiveName(name)
	_ = ic.cfg.Save()
	ic.SetConfigURL(rec.URL)
	ic.SetConfigName(name)
	ic.Log("Activated config: " + name)
	return nil
}

// UpdateAllConfigsWithLog updates all configs and logs progress.
func (ic *InteractiveController) UpdateAllConfigsWithLog(progress func(done, total int)) (int, int, error) {
	configs := ic.cfg.GetConfigs()
	total := 0
	for _, rec := range configs {
		if rec.URL != "" {
			total++
		}
	}
	if total == 0 {
		ic.Log("No configs to update")
		return 0, 0, nil
	}
	updated := 0
	for _, rec := range configs {
		if rec.URL == "" {
			continue
		}
		ic.Log("Updating config: " + rec.Name + "...")
		if err := ic.DownloadConfigFor(rec.Name, rec.URL); err != nil {
			ic.Log("Failed to update " + rec.Name + ": " + err.Error())
		} else {
			ic.cfg.SetLastUpdateFor(rec.Name, time.Now())
			updated++
			ic.Log("Config updated: " + rec.Name)
		}
		if progress != nil {
			progress(updated, total)
		}
	}
	_ = ic.cfg.Save()
	ic.Logf("Update all finished (%d/%d)", updated, total)
	return updated, total, nil
}

// PluginDiscoverWithLog discovers plugins and logs errors.
func (ic *InteractiveController) PluginDiscoverWithLog(pm interface{ Discover() error }) error {
	err := pm.Discover()
	if err != nil {
		ic.Log("[plugins] discover error: " + err.Error())
		return err
	}
	return nil
}

// PluginToggleWithLog toggles a plugin and logs the result.
func (ic *InteractiveController) PluginToggleWithLog(pm interface{ Toggle(string) error }, name string) error {
	err := pm.Toggle(name)
	if err != nil {
		ic.Log("[plugins] toggle failed: " + err.Error())
		return err
	}
	ic.Log("[plugins] toggled: " + name)
	return nil
}

// PluginCheckUpdateWithLog checks for plugin updates and logs the result.
func (ic *InteractiveController) PluginCheckUpdateWithLog(pm interface {
	CheckUpdate(string) (bool, string, error)
}, name string) (bool, string, error) {
	hasUpdate, latest, err := pm.CheckUpdate(name)
	if err != nil {
		ic.Log("[plugins] update check failed for " + name + ": " + err.Error())
		return false, "", err
	}
	if hasUpdate {
		ic.Log("[plugins] update available for " + name + ": v" + latest)
	} else {
		ic.Log("[plugins] " + name + " is up to date")
	}
	return hasUpdate, latest, nil
}

// PluginInstallFromURLWithLog installs a plugin from URL and logs the result.
func (ic *InteractiveController) PluginInstallFromURLWithLog(pm interface{ InstallFromURL(string) error }, url string) error {
	if url == "" {
		ic.Log("[plugins] install: URL is required")
		return fmt.Errorf("URL is required")
	}
	err := pm.InstallFromURL(url)
	if err != nil {
		ic.Log("[plugins] install failed: " + err.Error())
		return err
	}
	ic.Log("[plugins] installed from: " + url)
	return nil
}

// PluginGenerateTemplateWithLog generates a plugin template using the provided function and logs the result.
func (ic *InteractiveController) PluginGenerateTemplateWithLog(generateFunc func(outDir, name, rel string) error, outDir, name, rel string) error {
	if name == "" {
		ic.Log("[plugins] template: name is required")
		return fmt.Errorf("name is required")
	}
	if rel == "" {
		rel = "client"
	}
	if err := generateFunc(outDir, name, rel); err != nil {
		ic.Log("[plugins] template generation failed: " + err.Error())
		return err
	}
	ic.Log("[plugins] template generated: " + outDir)
	return nil
}

// PluginGenerateDocsWithLog generates plugin API docs using the provided function and logs the result.
func (ic *InteractiveController) PluginGenerateDocsWithLog(generateFunc func(outDir string) error, outDir string) error {
	if err := generateFunc(outDir); err != nil {
		ic.Log("[plugins] docs generation failed: " + err.Error())
		return err
	}
	ic.Log("[plugins] API docs generated: " + outDir)
	return nil
}

// PluginGenerateDefsWithLog generates VS Code Lua definitions using the provided function and logs the result.
func (ic *InteractiveController) PluginGenerateDefsWithLog(generateFunc func(outDir string) error, outDir string) error {
	if err := generateFunc(outDir); err != nil {
		ic.Log("[plugins] defs generation failed: " + err.Error())
		return err
	}
	ic.Log("[plugins] VS Code Lua defs generated: " + outDir)
	return nil
}

// PluginManagerLogCallback returns a logger callback suitable for plugins.Manager.
func (ic *InteractiveController) PluginManagerLogCallback() func(string) {
	return func(line string) {
		ic.Log(line)
	}
}

// ApplyPrivilegeAction executes a privilege action, logs the result, and returns
// whether it succeeded, whether the privilege UI should refresh, and whether the app should exit.
func (ic *InteractiveController) ApplyPrivilegeAction(action *PrivilegeAction) (success, needRefresh, needClose bool) {
	err := action.Handler()
	if err != nil {
		ic.Log(action.Label + " failed: " + err.Error())
		if action.ID == "setcap" {
			ic.Log("Tip: run manually: sudo setcap cap_net_admin=+ep ./sing-box")
		}
		return false, false, false
	}
	ic.Log(action.Label + " succeeded.")
	needRefresh = action.ID == "setcap" || action.ID == "run_as_admin"
	needClose = action.ID == "restart_admin"
	return true, needRefresh, needClose
}
