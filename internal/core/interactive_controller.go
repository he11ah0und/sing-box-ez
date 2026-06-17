package core

import (
	"errors"
	"sync"
	"time"

	"sing-box-ez/internal/framework"
	"sing-box-ez/internal/framework/localengine"
	"sing-box-ez/internal/framework/updater"
	"sing-box-ez/internal/framework/version"
)

// InteractiveController wraps Controller with GUI-specific callbacks and background loops.
type InteractiveController struct {
	Controller *Controller

	// UI callbacks (optional, invoked when state changes)
	OnStatusChange    func(running bool)
	OnLog             func(msg string)
	OnConfigUpdate    func()
	OnVersionChange   func(ver string)
	OnPrivilegeChange func(active bool)
	OnLatestVersion   func(ver string)
	OnNotification    func(title, body string)
	OnAutoRestart     func()
	// OnUpdateCheckDue is invoked periodically when a background update check
	// should be performed. The GUI layer is responsible for showing dialogs.
	OnUpdateCheckDue func()
	// OnCoreMissing is invoked when StartService fails because the sing-box
	// core binary is missing. The GUI layer can navigate to the Core page.
	OnCoreMissing func()
	// OnConfigMissing is invoked when StartService fails because no active
	// config is selected. The GUI layer can navigate to the Configs page.
	OnConfigMissing func()

	stopped bool
	stopMu  sync.Mutex
}

// NewInteractiveController creates an interactive controller wrapping an existing core controller.
func NewInteractiveController(c *Controller) *InteractiveController {
	cfg := c.Config()
	if lang := cfg.GetLanguage(); lang == "" {
		lang = localengine.DetectSystemLanguage()
		cfg.SetLanguage(lang)
		_ = cfg.Save()
		localengine.SetLanguage(lang)
	} else {
		localengine.SetLanguage(lang)
	}

	ic := &InteractiveController{
		Controller: c,
	}

	ic.Controller.LogProcessor().OnAutoRestart = func() {
		if ic.OnAutoRestart != nil {
			ic.OnAutoRestart()
		}
	}

	go ic.configUpdateChecker()
	go ic.statusChecker()
	go ic.updateCheckLoop()

	return ic
}

// Log logs a message and optionally invokes the OnLog callback.
func (ic *InteractiveController) Log(msg string) {
	ic.Controller.Terminal().Infof("%s", msg)
	if ic.OnLog != nil {
		ic.OnLog(msg)
	}
}

// App returns the framework application container.
func (ic *InteractiveController) App() *framework.App {
	return ic.Controller.Framework()
}

// SelfUpdater returns the updater manager used for the application binary.
func (ic *InteractiveController) SelfUpdater() *updater.Manager {
	for _, m := range ic.Controller.Framework().Updaters {
		if m.Name == "updater" {
			return m
		}
	}
	return nil
}

// CheckSelfUpdate checks whether a newer release of the app is available.
func (ic *InteractiveController) CheckSelfUpdate() (*updater.UpdateInfo, error) {
	return updater.CheckUpdate(version.Branch)
}

// CheckSelfUpdateForBranch checks for app updates on a specific branch.
func (ic *InteractiveController) CheckSelfUpdateForBranch(branch string) (*updater.UpdateInfo, error) {
	return updater.CheckUpdateForBranch(branch)
}

// GetBranches returns the list of available release channels.
func (ic *InteractiveController) GetBranches() ([]updater.Channel, error) {
	return updater.GetChannels()
}

// StartService prepares the active config and starts the core. It mirrors the
// main page start button action so other UI surfaces (e.g. tray) can reuse it.
func (ic *InteractiveController) StartService() error {
	if _, err := ic.Controller.PrepareConfig(); err != nil {
		ic.Controller.Terminal().Infof("%s", err.Error())
		switch {
		case errors.Is(err, ErrCoreMissing):
			if ic.OnCoreMissing != nil {
				ic.OnCoreMissing()
			}
		case errors.Is(err, ErrNoActiveConfig):
			if ic.OnConfigMissing != nil {
				ic.OnConfigMissing()
			}
		}
		return err
	}
	if err := ic.Controller.Start(); err != nil {
		ic.Controller.Terminal().Infof("Failed to start: %v", err)
		return err
	}
	return nil
}

// StopService stops the core. It mirrors the main page stop button action.
func (ic *InteractiveController) StopService() error {
	if err := ic.Controller.Stop(); err != nil {
		ic.Controller.Terminal().Infof("Failed to stop: %v", err)
		return err
	}
	return nil
}

func (ic *InteractiveController) isStopped() bool {
	ic.stopMu.Lock()
	defer ic.stopMu.Unlock()
	return ic.stopped
}

// Close stops background loops and the controller.
func (ic *InteractiveController) Close() {
	ic.stopMu.Lock()
	ic.stopped = true
	ic.stopMu.Unlock()
	ic.Controller.Close()
}

func (ic *InteractiveController) configUpdateChecker() {
	// Run an initial check right after startup so overdue configs are refreshed
	// before the first interval elapses.
	if ic.Controller.Config().GetAutoUpdateConfigs() {
		ic.checkAllConfigs()
	}

	for {
		interval := time.Duration(ic.Controller.Config().GetAutoUpdateConfigsIntervalHours()) * time.Hour
		if interval <= 0 {
			interval = time.Hour
		}

		select {
		case <-time.After(interval):
		}

		if ic.isStopped() {
			return
		}
		if !ic.Controller.Config().GetAutoUpdateConfigs() {
			continue
		}
		ic.checkAllConfigs()
	}
}

func (ic *InteractiveController) checkAllConfigs() {
	configs := ic.Controller.Config().GetConfigs()
	if len(configs) == 0 {
		return
	}

	active := ic.Controller.Config().GetActiveConfig()
	activeUpdated := false

	for i := range configs {
		cfg := &configs[i]
		if !cfg.ShouldUpdate() {
			continue
		}

		ic.Controller.Terminal().Infof("Auto-updating config: %s", cfg.Name)
		if err := ic.Controller.UpdateConfigNow(cfg.Name, cfg.URL); err != nil {
			ic.Controller.Terminal().Errorf("Auto-update failed for %s: %v", cfg.Name, err)
			continue
		}
		if active != nil && cfg.Name == active.Name {
			activeUpdated = true
		}
	}

	if activeUpdated {
		if ic.OnConfigUpdate != nil {
			ic.OnConfigUpdate()
		}
		if ic.Controller.Config().GetAutoRestartOnConfigUpdate() && ic.Controller.IsRunning() {
			ic.Controller.Terminal().Infof("Active config updated, restarting core...")
			if err := ic.Controller.Restart(); err != nil {
				ic.Controller.Terminal().Errorf("Auto-restart failed: %v", err)
			}
		}
	}
}

func (ic *InteractiveController) updateCheckLoop() {
	for {
		interval := time.Duration(ic.Controller.Config().GetBackgroundUpdateCheckIntervalHours()) * time.Hour
		if interval <= 0 {
			interval = 24 * time.Hour
		}

		select {
		case <-time.After(interval):
		}

		if ic.isStopped() {
			return
		}
		if ic.OnUpdateCheckDue != nil {
			ic.OnUpdateCheckDue()
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
		running := ic.Controller.IsRunning()
		if running != lastRunning {
			lastRunning = running
			if ic.OnStatusChange != nil {
				ic.OnStatusChange(running)
			}
		}
	}
}
