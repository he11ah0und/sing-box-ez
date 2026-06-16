package core

import (
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

	stopped bool
	stopMu  sync.Mutex
}

// NewInteractiveController creates an interactive controller wrapping an existing core controller.
func NewInteractiveController(c *Controller) *InteractiveController {
	cfg := c.Config()
	if !cfg.GetFirstRunDone() {
		lang := localengine.DetectSystemLanguage()
		cfg.SetLanguage(lang)
		_ = cfg.Save()
		localengine.SetLanguage(lang)
	} else if lang := cfg.GetLanguage(); lang != "" {
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

	go ic.updateChecker()
	go ic.statusChecker()

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

// RunStartupSequence performs first-run and update checks.
func (ic *InteractiveController) RunStartupSequence() {
	if !ic.Controller.Config().GetFirstRunDone() {
		if ic.OnFirstRun != nil {
			ic.OnFirstRun()
		}
	}
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

func (ic *InteractiveController) updateChecker() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		if ic.isStopped() {
			return
		}
		active := ic.Controller.Config().GetActiveConfig()
		if active != nil && active.ShouldUpdate() && ic.Controller.IsRunning() {
			ic.Controller.Terminal().Infof("Auto-updating config...")
			ic.Controller.Manager().SetConfigName(active.Name)
			if err := ic.Controller.UpdateConfig(); err != nil {
				ic.Controller.Terminal().Errorf("Auto-update failed: %v", err)
			} else {
				ic.Controller.Config().SetLastUpdateFor(active.Name, time.Now())
				_ = ic.Controller.Config().Save()
				if ic.OnConfigUpdate != nil {
					ic.OnConfigUpdate()
				}
				ic.Controller.Terminal().Infof("Config auto-updated, restarting core...")
				if err := ic.Controller.Restart(); err != nil {
					ic.Controller.Terminal().Errorf("Auto-restart failed: %v", err)
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
		running := ic.Controller.IsRunning()
		if running != lastRunning {
			lastRunning = running
			if ic.OnStatusChange != nil {
				ic.OnStatusChange(running)
			}
		}
	}
}
