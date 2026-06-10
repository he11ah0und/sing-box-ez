package core

import (
	"time"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/i18n"
	"sing-box-ez/internal/updater"
)

// InteractiveController is the high-level coordinator for GUI/TUI frontends.
// It owns specialised sub-controllers (Core, Config, Updater, Privileges, Plugins, App)
// and exposes UI callbacks for the presentation layer.
type InteractiveController struct {
	*Controller

	Core       *CoreController
	Configs    *ConfigController
	Updater    *UpdaterController
	Privileges *PrivilegeController
	Plugins    *PluginController
	App        *AppController

	// UI callbacks (optional, invoked when state changes)
	OnStatusChange        func(running bool)
	OnLog                 func(msg string)
	OnConfigUpdate        func()
	OnVersionChange       func(ver string)
	OnPrivilegeChange     func(active bool)
	OnLatestVersion       func(ver string)
	OnNotification        func(title, body string)
	OnAutoRestart         func()
	OnFirstRun            func()
	OnSelfUpdateAvailable func(info *updater.UpdateInfo)
}

// NewInteractiveController creates an interactive controller, initializes i18n,
// starts background maintenance loops, and wires sub-controllers.
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
	root := ctrl.LogRoot()

	ic := &InteractiveController{Controller: ctrl}
	ic.Core = NewCoreController(cfg, ctrl.Manager(), ctrl.Logger(), root.Allocate("core"))
	ic.Configs = NewConfigController(cfg, ctrl.Manager(), root.Allocate("config"))
	ic.Updater = NewUpdaterController(root.Allocate("updater"))
	ic.Privileges = NewPrivilegeController(cfg, ctrl.Manager(), root.Allocate("privileges"))
	ic.Plugins = NewPluginController(root.Allocate("plugins"))
	ic.App = NewAppController(cfg, root.Allocate("app"))

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

// Log logs a message from the root terminal and optionally invokes the OnLog callback.
func (ic *InteractiveController) Log(msg string) {
	ic.Controller.LogRoot().Info(msg)
	if ic.OnLog != nil {
		ic.OnLog(msg)
	}
}

// LogRoot returns the root logging terminal for allocating scoped log blocks.
func (ic *InteractiveController) LogRoot() *LogTerminal {
	return ic.Controller.LogRoot()
}

// RunStartupSequence performs first-run and update checks, invoking registered
// UI callbacks when user interaction is required.
func (ic *InteractiveController) RunStartupSequence() {
	if !ic.cfg.GetFirstRunDone() {
		if ic.OnFirstRun != nil {
			ic.OnFirstRun()
		}
	}
	info, err := ic.Updater.CheckSelfUpdate()
	if err == nil && info != nil && info.ReleaseCount > 0 {
		if ic.OnSelfUpdateAvailable != nil {
			ic.OnSelfUpdateAvailable(info)
		}
	}
}

// ---------- Background loops ----------

func (ic *InteractiveController) updateChecker() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		if ic.isStopped() {
			return
		}
		active := ic.Controller.Config().GetActiveConfig()
		if active != nil && active.ShouldUpdate() && ic.IsRunning() {
			ic.Configs.Terminal().Info("Auto-updating config...")
			ic.Manager().SetConfigName(active.Name)
			if err := ic.UpdateConfig(); err != nil {
				ic.Configs.Terminal().Error("Auto-update failed: " + err.Error())
			} else {
				ic.Controller.Config().SetLastUpdateFor(active.Name, time.Now())
				_ = ic.Controller.Config().Save()
				if ic.OnConfigUpdate != nil {
					ic.OnConfigUpdate()
				}
				ic.Configs.Terminal().Info("Config auto-updated, restarting core...")
				if err := ic.Core.RestartCore(); err != nil {
					ic.Core.Terminal().Error("Auto-restart failed: " + err.Error())
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

// ---------- Forward methods for backwards compatibility ----------

func (ic *InteractiveController) HasRequiredPrivileges() bool {
	return ic.Privileges.HasRequiredPrivileges()
}

func (ic *InteractiveController) RefreshPrivilegeStatus() string {
	return ic.Privileges.RefreshPrivilegeStatus()
}

func (ic *InteractiveController) GetInstalledCoreVersion() (string, error) {
	return ic.Core.GetInstalledCoreVersion()
}

func (ic *InteractiveController) GetLatestCoreVersion() (string, error) {
	return ic.Core.GetLatestCoreVersion()
}

func (ic *InteractiveController) DownloadCoreWithProgress(onProgress func(downloaded, total int64)) (string, error) {
	return ic.Core.DownloadCoreWithProgress(onProgress)
}

func (ic *InteractiveController) CheckSelfUpdate() (*updater.UpdateInfo, error) {
	return ic.Updater.CheckSelfUpdate()
}

func (ic *InteractiveController) GetPrivilegeDialog() *PrivilegeDialog {
	return ic.Privileges.GetPrivilegeDialog(ic.Core.RestartAsAdmin)
}

func (ic *InteractiveController) GetPrivilegeTabState() PrivilegeTabState {
	return ic.Privileges.GetPrivilegeTabState()
}

func (ic *InteractiveController) CoreExists() bool {
	return ic.Core.CoreExists()
}

func (ic *InteractiveController) ApplySetcap() error {
	return ic.Privileges.ApplySetcap()
}

func (ic *InteractiveController) ApplySelfUpdate(assetURL string, onProgress func(int64, int64)) error {
	return ic.Updater.ApplySelfUpdate(assetURL, onProgress)
}

func (ic *InteractiveController) HasCachedConfig(name string) bool {
	return ic.Configs.HasCachedConfig(name)
}

func (ic *InteractiveController) DownloadConfigFor(name, url string) error {
	return ic.Configs.DownloadConfigFor(name, url)
}

func (ic *InteractiveController) StartCore() error {
	return ic.Core.StartCore()
}

func (ic *InteractiveController) StopCore() error {
	return ic.Core.StopCore()
}

func (ic *InteractiveController) RestartCore() error {
	return ic.Core.RestartCore()
}

func (ic *InteractiveController) GetLatestCoreVersionWithLog() (string, error) {
	return ic.Core.GetLatestCoreVersionWithLog()
}

func (ic *InteractiveController) RestartAsAdminWithLog() error {
	return ic.Privileges.RestartAsAdminWithLog(ic.Core.RestartAsAdmin)
}

func (ic *InteractiveController) SetRunAsAdminWithLog(checked bool) error {
	return ic.Privileges.SetRunAsAdminWithLog(checked)
}

func (ic *InteractiveController) ApplySetcapWithLog() error {
	return ic.Privileges.ApplySetcapWithLog()
}

func (ic *InteractiveController) DownloadCoreWithProgressWithLog(onProgress func(int64, int64)) (string, error) {
	return ic.Core.DownloadCoreWithProgressWithLog(onProgress)
}

func (ic *InteractiveController) ApplySelfUpdateWithLog(assetURL string, onProgress func(int64, int64)) error {
	return ic.Updater.ApplySelfUpdateWithLog(assetURL, onProgress)
}

func (ic *InteractiveController) AddFirstConfigWithLog(name, url string) error {
	return ic.Configs.AddFirstConfigWithLog(name, url)
}

func (ic *InteractiveController) CheckSelfUpdateWithLog() (*updater.UpdateInfo, error) {
	return ic.Updater.CheckSelfUpdateWithLog()
}

func (ic *InteractiveController) GetBranches() ([]updater.Branch, error) {
	return ic.Updater.GetBranches()
}

func (ic *InteractiveController) CheckSelfUpdateForBranch(branch string) (*updater.UpdateInfo, error) {
	return ic.Updater.CheckSelfUpdateForBranch(branch)
}

func (ic *InteractiveController) OpenDataFolderWithLog() error {
	return ic.App.OpenDataFolderWithLog()
}

func (ic *InteractiveController) FetchReleaseNotesWithLog(commit string) (updater.Release, error) {
	return ic.Updater.FetchReleaseNotesWithLog(commit)
}

func (ic *InteractiveController) SetLogLimitWithLog(v int) {
	ic.App.SetLogLimitWithLog(v)
}

func (ic *InteractiveController) SetDefaultIntervalWithLog(h int) {
	ic.App.SetDefaultIntervalWithLog(h)
}

func (ic *InteractiveController) UpdateConfigNowWithLog(name, url string) error {
	return ic.Configs.UpdateConfigNowWithLog(name, url)
}

func (ic *InteractiveController) AddConfigWithLog(rec config.ConfigRecord) error {
	return ic.Configs.AddConfigWithLog(rec)
}

func (ic *InteractiveController) EditConfigWithLog(oldName string, rec config.ConfigRecord) error {
	return ic.Configs.EditConfigWithLog(oldName, rec)
}

func (ic *InteractiveController) DeleteConfigWithLog(name string) error {
	return ic.Configs.DeleteConfigWithLog(name)
}

func (ic *InteractiveController) ActivateConfigWithLog(name string) error {
	return ic.Configs.ActivateConfigWithLog(name)
}

func (ic *InteractiveController) UpdateAllConfigsWithLog(progress func(done, total int)) (int, int, error) {
	return ic.Configs.UpdateAllConfigsWithLog(progress)
}

func (ic *InteractiveController) PluginDiscoverWithLog(pm interface{ Discover() error }) error {
	return ic.Plugins.Discover(pm)
}

func (ic *InteractiveController) PluginToggleWithLog(pm interface{ Toggle(string) error }, name string) error {
	return ic.Plugins.Toggle(pm, name)
}

func (ic *InteractiveController) PluginCheckUpdateWithLog(pm interface {
	CheckUpdate(string) (bool, string, error)
}, name string) (bool, string, error) {
	return ic.Plugins.CheckUpdate(pm, name)
}

func (ic *InteractiveController) PluginInstallFromURLWithLog(pm interface{ InstallFromURL(string) error }, url string) error {
	return ic.Plugins.InstallFromURL(pm, url)
}

func (ic *InteractiveController) PluginGenerateTemplateWithLog(generateFunc func(outDir, name, rel string) error, outDir, name, rel string) error {
	return ic.Plugins.GenerateTemplate(generateFunc, outDir, name, rel)
}

func (ic *InteractiveController) PluginGenerateDocsWithLog(generateFunc func(outDir string) error, outDir string) error {
	return ic.Plugins.GenerateDocs(generateFunc, outDir)
}

func (ic *InteractiveController) PluginGenerateDefsWithLog(generateFunc func(outDir string) error, outDir string) error {
	return ic.Plugins.GenerateDefs(generateFunc, outDir)
}

func (ic *InteractiveController) PluginManagerLogCallback() func(string) {
	return ic.Plugins.ManagerLogCallback()
}

func (ic *InteractiveController) ApplyPrivilegeAction(action *PrivilegeAction) (success, needRefresh, needClose bool) {
	return ic.Privileges.ApplyPrivilegeAction(action)
}

// LogTag logs a message with a subsystem tag for simple backwards-compatible usage.
func (ic *InteractiveController) LogTag(tag, msg string) {
	switch tag {
	case "core":
		ic.Core.Terminal().Info(msg)
	case "config":
		ic.Configs.Terminal().Info(msg)
	case "updater":
		ic.Updater.Terminal().Info(msg)
	case "privileges":
		ic.Privileges.Terminal().Info(msg)
	case "plugins":
		ic.Plugins.Terminal().Info(msg)
	case "app":
		ic.App.Terminal().Info(msg)
	default:
		ic.LogRoot().Info(msg)
	}
}
