package core

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/he11ah0und/localengine"
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core/inboundstyle"
	"sing-box-ez/internal/core/state"
	"sing-box-ez/internal/framework"
	"sing-box-ez/internal/framework/svcman"
	"sing-box-ez/internal/framework/updater"
	"sing-box-ez/internal/framework/version"
)

// ConfigUpdateFailure describes one config that failed its background update.
type ConfigUpdateFailure struct {
	Name string `json:"name"`
	// Kind is one of the Failure* constants (refused/timeout/dns/http/other).
	Kind string `json:"kind"`
	// LastUpdate is the last successful refresh ("2006-01-02 15:04"), empty
	// when the config was never updated.
	LastUpdate string `json:"lastUpdate,omitempty"`
	// LastUpdateAgo is LastUpdate humanized ("3 days ago"), empty when unknown.
	LastUpdateAgo string `json:"lastUpdateAgo,omitempty"`
}

// ConfigsUpdateReport summarizes a background update run in which every
// config due for a refresh failed: Attempted configs were due (all of them
// are in Failures), out of Total remote configs in the list. The UI needs
// both numbers: one due config failing reads very differently from the whole
// list failing.
type ConfigsUpdateReport struct {
	Attempted int                   `json:"attempted"`
	Total     int                   `json:"total"`
	Failures  []ConfigUpdateFailure `json:"failures"`
}

func newConfigUpdateFailure(cfg *config.ConfigRecord, err error) ConfigUpdateFailure {
	f := ConfigUpdateFailure{Name: cfg.Name, Kind: DownloadErrorKind(err)}
	if !cfg.LastUpdate.IsZero() {
		f.LastUpdate = cfg.LastUpdate.Format("2006-01-02 15:04")
		f.LastUpdateAgo = version.HumanDuration(cfg.LastUpdate.Time)
	}
	return f
}

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
	// OnCoreIncompatible is invoked when StartService fails because the
	// selected core binary is older than MinCoreVersion. The GUI layer can
	// navigate to the core settings.
	OnCoreIncompatible func()
	// OnConfigStyleCheck is invoked when the active config does not look like a
	// client config and no fallback_type has been chosen. The GUI should show a
	// dialog and call choose with "ignore" or "to_client".
	OnConfigStyleCheck func(style inboundstyle.Style, rec *config.ConfigRecord, choose func(string))
	// OnPhaseChange is invoked with the current start-flow stage (see the
	// Phase* constants in internal/core/state) so the UI can show granular
	// progress while StartService runs.
	OnPhaseChange func(phase string)
	// OnConfigsUpdateFailed is invoked after a background config update run in
	// which every config due for an update failed to download (e.g. no direct
	// connectivity). The report carries attempted/total counts so the UI can
	// tell "the only due config failed" from "the whole list failed".
	OnConfigsUpdateFailed func(report ConfigsUpdateReport)

	stopped bool
	stopMu  sync.Mutex

	// startMu guards startCancel: the cancel func of the in-flight
	// StartService context, set for the duration of the start flow.
	startMu     sync.Mutex
	startCancel context.CancelFunc
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
		_ = cfg.MustGet("ui", "language").Update(lang)
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
	ic.backend.Terminal().TInfof("core.interactive.log_message", msg)
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
// Stages are reported through OnPhaseChange.
func (ic *InteractiveController) StartService() error {
	ctx, cancel := context.WithCancel(context.Background())
	ic.startMu.Lock()
	ic.startCancel = cancel
	ic.startMu.Unlock()
	defer func() {
		cancel()
		ic.startMu.Lock()
		ic.startCancel = nil
		ic.startMu.Unlock()
	}()

	ic.reportPhase(state.PhasePreparingConfig)
	defer ic.reportPhase("")
	rec, err := ic.backend.PrepareConfig(ctx)
	if err != nil {
		if ctx.Err() != nil || errors.Is(err, ErrStartCancelled) {
			ic.backend.Terminal().TInfof("core.interactive.start_cancelled")
			return ErrStartCancelled
		}
		ic.handlePrepareConfigError(err)
		return err
	}

	if ic.Controller != nil && rec != nil {
		if ctx.Err() != nil {
			ic.backend.Terminal().TInfof("core.interactive.start_cancelled")
			return ErrStartCancelled
		}
		ic.reportPhase(state.PhaseCheckingConfig)
		if err := ic.checkClientStyle(rec); err != nil {
			return err
		}
	}

	ic.reportPhase(state.PhaseStarting)
	if err := ic.startBackend(); err != nil {
		return err
	}
	// A cancel that landed while the process was spawning must not leave the
	// core running.
	if ctx.Err() != nil {
		ic.backend.Terminal().TInfof("core.interactive.start_cancelled")
		_ = ic.StopService()
		return ErrStartCancelled
	}
	return nil
}

// CancelStart aborts an in-flight StartService (config download, style
// check, process spawn) and stops the core if it already came up. Safe to
// call when no start is in flight: it then acts as a plain stop.
func (ic *InteractiveController) CancelStart() {
	ic.startMu.Lock()
	cancel := ic.startCancel
	ic.startMu.Unlock()
	if cancel != nil {
		cancel()
	}
	if ic.backend.IsRunning() {
		if err := ic.StopService(); err != nil {
			ic.backend.Terminal().TInfof("core.interactive.stop_failed", err)
		}
	}
}

// StartInFlight reports whether a StartService flow is currently running.
func (ic *InteractiveController) StartInFlight() bool {
	ic.startMu.Lock()
	defer ic.startMu.Unlock()
	return ic.startCancel != nil
}

// reportPhase forwards the start-flow stage to the UI callback, if set.
func (ic *InteractiveController) reportPhase(phase string) {
	if ic.OnPhaseChange != nil {
		ic.OnPhaseChange(phase)
	}
}

func (ic *InteractiveController) handlePrepareConfigError(err error) {
	switch {
	case errors.Is(err, ErrCoreMissing):
		if ic.OnCoreMissing != nil {
			ic.OnCoreMissing()
		}
	case errors.Is(err, ErrNoActiveConfig):
		if ic.OnConfigMissing != nil {
			ic.OnConfigMissing()
		}
	case errors.Is(err, ErrCoreIncompatible):
		if ic.OnCoreIncompatible != nil {
			ic.OnCoreIncompatible()
		}
	}
}

func (ic *InteractiveController) checkClientStyle(rec *config.ConfigRecord) error {
	style, err := ic.Controller.DetectConfigStyle(rec.Name)
	if err != nil {
		ic.backend.Terminal().TInfof("core.interactive.detect_style_failed", err)
		return fmt.Errorf("detect config style: %w", err)
	}
	if style == inboundstyle.StyleClient || rec.GetFallbackType() != "" {
		return nil
	}
	if ic.OnConfigStyleCheck != nil {
		ic.OnConfigStyleCheck(style, rec, func(fallbackType string) {
			if err := ic.Controller.SetFallbackType(rec.Name, fallbackType); err != nil {
				ic.backend.Terminal().TInfof("core.interactive.set_fallback_type_failed", err)
				return
			}
			_ = ic.StartService()
		})
		return nil
	}
	return fmt.Errorf("config %q is not a client config; set fallback_type to %q or %q in the profile", rec.Name, inboundstyle.FallbackIgnore, inboundstyle.FallbackToClient)
}

func (ic *InteractiveController) startBackend() error {
	if ic.serviceManager != nil {
		if err := ic.serviceManager.Start(); err != nil {
			ic.backend.Terminal().TInfof("core.interactive.start_failed", err)
			return err
		}
		return nil
	}

	if err := ic.backend.Start(); err != nil {
		ic.backend.Terminal().TInfof("core.interactive.start_failed", err)
		return err
	}
	return nil
}

// StopService stops the core. It mirrors the main page stop button action.
func (ic *InteractiveController) StopService() error {
	if ic.serviceManager != nil {
		if err := ic.serviceManager.Stop(); err != nil {
			ic.backend.Terminal().TInfof("core.interactive.stop_failed", err)
			return err
		}
		return nil
	}

	if err := ic.backend.Stop(); err != nil {
		ic.backend.Terminal().TInfof("core.interactive.stop_failed", err)
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
	activeUpdated, report := ic.updateOutdatedConfigs(configs, active)

	if activeUpdated {
		ic.onActiveConfigUpdated()
	}
	// Notify the UI only when every config that was due for an update failed:
	// partial failures are already logged per config and stay silent.
	if report != nil && ic.OnConfigsUpdateFailed != nil {
		ic.OnConfigsUpdateFailed(*report)
	}
}

func (ic *InteractiveController) updateOutdatedConfigs(configs []config.ConfigRecord, active *config.ConfigRecord) (bool, *ConfigsUpdateReport) {
	autoUpdateConfigs := ic.backend.Config().MustGet("updates", "auto_update_configs").Bool()
	autoUpdateOnHashMismatch := ic.backend.Config().MustGet("updates", "auto_update_on_hash_mismatch").Bool()

	activeUpdated := false
	var failures []ConfigUpdateFailure
	dueCount := 0
	total := 0
	for i := range configs {
		cfg := &configs[i]
		if cfg.IsLocal() {
			continue
		}
		total++
		attempted, updated, err := ic.tryUpdateConfig(cfg, active, autoUpdateConfigs, autoUpdateOnHashMismatch)
		if !attempted {
			continue
		}
		dueCount++
		if err != nil {
			failures = append(failures, newConfigUpdateFailure(cfg, err))
			continue
		}
		if updated {
			activeUpdated = true
		}
	}
	// Only an across-the-board wipeout is worth a dialog: every config that
	// was due failed. The report carries due vs total counts so the UI does
	// not read "one due config failed" as "no config works at all".
	if dueCount == 0 || len(failures) < dueCount {
		return activeUpdated, nil
	}
	return activeUpdated, &ConfigsUpdateReport{Attempted: dueCount, Total: total, Failures: failures}
}

// tryUpdateConfig reports whether an update was attempted at all (the config
// was due), whether the attempt refreshed the active profile, and the failure.
func (ic *InteractiveController) tryUpdateConfig(cfg *config.ConfigRecord, active *config.ConfigRecord, autoUpdateConfigs, autoUpdateOnHashMismatch bool) (attempted, activeUpdated bool, err error) {
	if ic.Controller != nil {
		// The due check repeats under the controller's config update lock:
		// a start flow or manual update that already refreshed the profile
		// makes this a no-op instead of a redundant download + restart.
		updated, err := ic.Controller.UpdateConfigIfDue(cfg.Name, autoUpdateConfigs, autoUpdateOnHashMismatch)
		if err != nil {
			// The network backend already logs the failure with context.
			return true, false, err
		}
		if !updated {
			return false, false, nil
		}
		return true, active != nil && cfg.Name == active.Name, nil
	}

	needsUpdate := autoUpdateConfigs && cfg.ShouldUpdate()
	needsHashUpdate := autoUpdateOnHashMismatch && ic.backend.IsConfigHashMismatch(cfg.Name)
	if !needsUpdate && !needsHashUpdate {
		return false, false, nil
	}

	ic.backend.Terminal().TInfof("core.interactive.auto_updating_config", cfg.Name)
	if err := ic.backend.UpdateConfigNow(cfg.Name, cfg.URL); err != nil {
		// The download/network backend already logs the failure with context.
		return true, false, err
	}
	return true, active != nil && cfg.Name == active.Name, nil
}

func (ic *InteractiveController) onActiveConfigUpdated() {
	if ic.OnConfigUpdate != nil {
		ic.OnConfigUpdate()
	}
	if !ic.backend.Config().MustGet("updates", "auto_restart_on_config_update").Bool() || !ic.backend.IsRunning() {
		return
	}
	// A start in flight already loads the freshest config via PrepareConfig;
	// restarting now would tear down the API the UI just connected to.
	if ic.StartInFlight() {
		ic.backend.Terminal().TInfof("core.interactive.auto_restart_skipped_start_in_flight")
		return
	}
	ic.backend.Terminal().TInfof("core.interactive.active_config_updated_restarting")
	// Report the restart as a phase so the UI shows "restarting" instead of an
	// unexplained drop back to waiting_api.
	ic.reportPhase(state.PhaseRestarting)
	defer ic.reportPhase("")
	if err := ic.backend.Restart(); err != nil {
		_ = ic.backend.Terminal().TErrorf("core.auto_restart_failed", err)
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
