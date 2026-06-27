package core

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core/inboundstyle"
	"sing-box-ez/internal/framework"
	"sing-box-ez/internal/framework/localengine"
	"sing-box-ez/internal/framework/svcman"
	"sing-box-ez/internal/framework/updater"
	"sing-box-ez/internal/framework/version"
)

// InteractiveController wraps Backend with GUI-specific callbacks and background loops.
type InteractiveController struct {
	// backend is the core backend used for all operations.
	backend Backend

	// Controller is the local controller, set only when running in local/embed/service mode.
	Controller *Controller

	// serviceManager controls the core lifecycle. In embed mode it is nil and the
	// backend is used directly; in service mode it points to a system service manager.
	serviceManager svcman.Manager

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
	// OnConfigStyleCheck is invoked when the active config does not look like a
	// client config and no fallback_type has been chosen. The GUI should show a
	// dialog and call choose with "ignore" or "to_client".
	OnConfigStyleCheck func(style inboundstyle.Style, rec *config.ConfigRecord, choose func(string))

	stopped bool
	stopMu  sync.Mutex
}

// NewInteractiveController creates an interactive controller wrapping a backend.
func NewInteractiveController(b Backend) *InteractiveController {
	return NewInteractiveControllerWithManager(b, nil)
}

// NewInteractiveControllerWithManager creates an interactive controller that
// uses the provided service manager. If manager is nil, backend.Start/Stop are
// used directly.
func NewInteractiveControllerWithManager(b Backend, manager svcman.Manager) *InteractiveController {
	cfg := b.Config()
	if lang := cfg.MustGet("ui", "language").String(); lang == "" {
		lang = localengine.DetectSystemLanguage()
		cfg.MustGet("ui", "language").Update(lang)
		_ = cfg.Save()
		localengine.SetLanguage(lang)
	} else {
		localengine.SetLanguage(lang)
	}

	ic := &InteractiveController{
		backend:        b,
		serviceManager: manager,
	}

	if c, ok := b.(*Controller); ok {
		ic.Controller = c
		c.LogProcessor().OnAutoRestart = func() {
			if ic.OnAutoRestart != nil {
				ic.OnAutoRestart()
			}
		}
	}

	go ic.configUpdateChecker()
	go ic.statusChecker()
	go ic.updateCheckLoop()

	return ic
}

// Log logs a message and optionally invokes the OnLog callback.
func (ic *InteractiveController) Log(msg string) {
	ic.backend.Terminal().Infof("%s", msg)
	if ic.OnLog != nil {
		ic.OnLog(msg)
	}
}

// Backend returns the core backend used by the interactive controller.
func (ic *InteractiveController) Backend() Backend {
	return ic.backend
}

// App returns the framework application container.
func (ic *InteractiveController) App() *framework.App {
	if ic.Controller == nil {
		return nil
	}
	return ic.Controller.Framework()
}

// SelfUpdater returns the updater manager used for the application binary.
func (ic *InteractiveController) SelfUpdater() *updater.Manager {
	app := ic.App()
	if app == nil {
		return nil
	}
	for _, m := range app.Updaters {
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
	rec, err := ic.backend.PrepareConfig()
	if err != nil {
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

	if ic.Controller != nil && rec != nil {
		style, err := ic.Controller.DetectConfigStyle(rec.Name)
		if err != nil {
			ic.backend.Terminal().Infof("Failed to detect config style: %v", err)
			return fmt.Errorf("detect config style: %w", err)
		}
		if style != inboundstyle.StyleClient && rec.GetFallbackType() == "" {
			if ic.OnConfigStyleCheck != nil {
				ic.OnConfigStyleCheck(style, rec, func(fallbackType string) {
					if err := ic.Controller.SetFallbackType(rec.Name, fallbackType); err != nil {
						ic.backend.Terminal().Infof("Failed to set fallback_type: %v", err)
						return
					}
					_ = ic.StartService()
				})
				return nil
			}
			return fmt.Errorf("config %q is not a client config; set fallback_type to %q or %q in the profile", rec.Name, inboundstyle.FallbackIgnore, inboundstyle.FallbackToClient)
		}
	}

	if ic.serviceManager != nil {
		if err := ic.serviceManager.Start(); err != nil {
			ic.backend.Terminal().Infof("Failed to start: %v", err)
			return err
		}
		return nil
	}

	if err := ic.backend.Start(); err != nil {
		ic.backend.Terminal().Infof("Failed to start: %v", err)
		return err
	}
	return nil
}

// StopService stops the core. It mirrors the main page stop button action.
func (ic *InteractiveController) StopService() error {
	if ic.serviceManager != nil {
		if err := ic.serviceManager.Stop(); err != nil {
			ic.backend.Terminal().Infof("Failed to stop: %v", err)
			return err
		}
		return nil
	}

	if err := ic.backend.Stop(); err != nil {
		ic.backend.Terminal().Infof("Failed to stop: %v", err)
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
	if ic.Controller != nil {
		ic.Controller.Close()
	}
}

func (ic *InteractiveController) configUpdateChecker() {
	// Run an initial check right after startup so overdue configs are refreshed
	// before the first interval elapses.
	if ic.backend.Config().MustGet("updates", "auto_update_configs").Bool() ||
		ic.backend.Config().MustGet("updates", "auto_update_on_hash_mismatch").Bool() {
		ic.checkAllConfigs()
	}

	for {
		interval := time.Duration(ic.backend.Config().MustGet("updates", "auto_update_configs_interval_hours").Int()) * time.Hour
		if interval <= 0 {
			interval = time.Hour
		}

		time.Sleep(interval)

		if ic.isStopped() {
			return
		}
		if !ic.backend.Config().MustGet("updates", "auto_update_configs").Bool() &&
			!ic.backend.Config().MustGet("updates", "auto_update_on_hash_mismatch").Bool() {
			continue
		}
		ic.checkAllConfigs()
	}
}

func (ic *InteractiveController) checkAllConfigs() {
	configs := ic.backend.GetConfigs()
	if len(configs) == 0 {
		return
	}

	active := ic.backend.GetActiveConfig()
	activeUpdated := false

	autoUpdateConfigs := ic.backend.Config().MustGet("updates", "auto_update_configs").Bool()
	autoUpdateOnHashMismatch := ic.backend.Config().MustGet("updates", "auto_update_on_hash_mismatch").Bool()

	for i := range configs {
		cfg := &configs[i]
		if cfg.IsLocal() {
			continue
		}
		needsUpdate := autoUpdateConfigs && cfg.ShouldUpdate()
		needsHashUpdate := autoUpdateOnHashMismatch && ic.backend.IsConfigHashMismatch(cfg.Name)
		if !needsUpdate && !needsHashUpdate {
			continue
		}

		ic.backend.Terminal().Infof("Auto-updating config: %s", cfg.Name)
		if err := ic.backend.UpdateConfigNow(cfg.Name, cfg.URL); err != nil {
			// The download/network backend already logs the failure with context.
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
		if ic.backend.Config().MustGet("updates", "auto_restart_on_config_update").Bool() && ic.backend.IsRunning() {
			ic.backend.Terminal().Infof("Active config updated, restarting core...")
			if err := ic.backend.Restart(); err != nil {
				ic.backend.Terminal().Errorf("Auto-restart failed: %v", err)
			}
		}
	}
}

func (ic *InteractiveController) updateCheckLoop() {
	for {
		interval := time.Duration(ic.backend.Config().MustGet("updates", "background_update_check_interval_hours").Int()) * time.Hour
		if interval <= 0 {
			interval = 24 * time.Hour
		}

		time.Sleep(interval)

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
		running := ic.backend.IsRunning()
		if running != lastRunning {
			lastRunning = running
			if ic.OnStatusChange != nil {
				ic.OnStatusChange(running)
			}
		}
	}
}
