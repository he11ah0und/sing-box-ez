package core

import (
	"time"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/framework"
	"sing-box-ez/internal/framework/localengine"
	"sing-box-ez/internal/framework/logger"
	"sing-box-ez/internal/framework/updater"
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
func NewInteractiveController(cfg *config.AppConfig, fwApp *framework.App, terminal *logger.LogTerminal) *InteractiveController {
	// Initialize i18n
	if !cfg.GetFirstRunDone() {
		lang := localengine.DetectSystemLanguage()
		cfg.SetLanguage(lang)
		_ = cfg.Save()
		localengine.SetLanguage(lang)
	} else if lang := cfg.GetLanguage(); lang != "" {
		localengine.SetLanguage(lang)
	}

	ctrl := NewController(cfg, fwApp, terminal)

	ic := &InteractiveController{Controller: ctrl}
	ic.Core = NewCoreController(cfg, ctrl.Manager(), fwApp.Logger, terminal)
	ic.Configs = NewConfigController(cfg, ctrl.Manager(), terminal)
	ic.Updater = NewUpdaterController(fwApp, terminal)
	ic.Privileges = NewPrivilegeController(cfg, ctrl.Manager(), terminal)
	ic.Plugins = NewPluginController(terminal)
	ic.App = NewAppController(cfg, fwApp.Logger, terminal, fwApp)

	ctrl.LogProcessor().OnAutoRestart = func() {
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
	ic.Controller.Terminal().Infof("%s", msg)
	if ic.OnLog != nil {
		ic.OnLog(msg)
	}
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
			ic.Configs.Terminal().Infof("Auto-updating config...")
			ic.Manager().SetConfigName(active.Name)
			if err := ic.UpdateConfig(); err != nil {
				ic.Configs.Terminal().Errorf("Auto-update failed: %v", err)
			} else {
				ic.Controller.Config().SetLastUpdateFor(active.Name, time.Now())
				_ = ic.Controller.Config().Save()
				if ic.OnConfigUpdate != nil {
					ic.OnConfigUpdate()
				}
				ic.Configs.Terminal().Infof("Config auto-updated, restarting core...")
				if err := ic.Core.RestartCore(); err != nil {
					ic.Core.Terminal().Errorf("Auto-restart failed: %v", err)
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

func (ic *InteractiveController) ApplySelfUpdate(assetURL string, onProgress func(downloaded, total int64)) error {
	return ic.Updater.ApplySelfUpdate(assetURL, onProgress)
}

func (ic *InteractiveController) HasCachedConfig(name string) bool {
	return ic.Configs.HasCachedConfig(name)
}

func (ic *InteractiveController) DownloadConfigFor(name, url string) error {
	return ic.Configs.DownloadConfigFor(name, url)
}

func (ic *InteractiveController) AddFirstConfig(name, url string) error {
	return ic.Configs.AddFirstConfig(name, url)
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

func (ic *InteractiveController) RestartAsAdmin() error {
	return ic.Privileges.RestartAsAdmin(ic.Core.RestartAsAdmin)
}

func (ic *InteractiveController) SetRunAsAdmin(checked bool) error {
	return ic.Privileges.SetRunAsAdmin(checked)
}

func (ic *InteractiveController) GetBranches() ([]updater.Channel, error) {
	return ic.Updater.GetBranches()
}

func (ic *InteractiveController) CheckSelfUpdateForBranch(branch string) (*updater.UpdateInfo, error) {
	return ic.Updater.CheckSelfUpdateForBranch(branch)
}

func (ic *InteractiveController) OpenDataFolder() error {
	return ic.App.OpenDataFolder()
}

func (ic *InteractiveController) FetchReleaseNotes(commit string) (updater.Release, error) {
	return ic.Updater.FetchReleaseNotes(commit)
}

func (ic *InteractiveController) SetLogLimit(v int) {
	ic.App.SetLogLimit(v)
}

func (ic *InteractiveController) SetDefaultInterval(h int) {
	ic.App.SetDefaultInterval(h)
}

func (ic *InteractiveController) UpdateConfigNow(name, url string) error {
	return ic.Configs.UpdateConfigNow(name, url)
}

func (ic *InteractiveController) AddConfig(rec config.ConfigRecord) error {
	return ic.Configs.AddConfig(rec)
}

func (ic *InteractiveController) EditConfig(oldName string, rec config.ConfigRecord) error {
	return ic.Configs.EditConfig(oldName, rec)
}

func (ic *InteractiveController) DeleteConfig(name string) error {
	return ic.Configs.DeleteConfig(name)
}

func (ic *InteractiveController) ActivateConfig(name string) error {
	return ic.Configs.ActivateConfig(name)
}

func (ic *InteractiveController) UpdateAllConfigs(progress func(done, total int)) (int, int, error) {
	return ic.Configs.UpdateAllConfigs(progress)
}

func (ic *InteractiveController) PluginDiscover(pm interface{ Discover() error }) error {
	return ic.Plugins.Discover(pm)
}

func (ic *InteractiveController) PluginToggle(pm interface{ Toggle(string) error }, name string) error {
	return ic.Plugins.Toggle(pm, name)
}

func (ic *InteractiveController) PluginCheckUpdate(pm interface {
	CheckUpdate(string) (bool, string, error)
}, name string) (bool, string, error) {
	return ic.Plugins.CheckUpdate(pm, name)
}

func (ic *InteractiveController) PluginInstallFromURL(pm interface{ InstallFromURL(string) error }, url string) error {
	return ic.Plugins.InstallFromURL(pm, url)
}

func (ic *InteractiveController) PluginGenerateTemplate(generateFunc func(outDir, name, rel string) error, outDir, name, rel string) error {
	return ic.Plugins.GenerateTemplate(generateFunc, outDir, name, rel)
}

func (ic *InteractiveController) PluginGenerateDocs(generateFunc func(outDir string) error, outDir string) error {
	return ic.Plugins.GenerateDocs(generateFunc, outDir)
}

func (ic *InteractiveController) PluginGenerateDefs(generateFunc func(outDir string) error, outDir string) error {
	return ic.Plugins.GenerateDefs(generateFunc, outDir)
}

func (ic *InteractiveController) PluginManagerLogCallback() func(string) {
	return ic.Plugins.ManagerLogCallback()
}

func (ic *InteractiveController) ApplyPrivilegeAction(action *PrivilegeAction) (success, needRefresh, needClose bool) {
	return ic.Privileges.ApplyPrivilegeAction(action)
}

