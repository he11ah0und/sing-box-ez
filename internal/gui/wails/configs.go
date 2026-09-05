//go:build !nogui

package wails

import (
	"fmt"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"sing-box-ez/internal/core"
	"sing-box-ez/internal/core/inboundstyle"
	"sing-box-ez/internal/framework/version"
	"sing-box-ez/internal/singboxconfig"
)

// ConfigMeta carries per-profile display metadata for the configs list:
// the detected inbound style and the last/next update times as compact
// plain durations ("2h", "3h 5m"). Empty strings mean "no value to show".
type ConfigMeta struct {
	// Style is "client" or "server"; empty for undefined or unreadable configs.
	Style string `json:"style"`
	// LastPlain is the elapsed time since the last successful update; empty
	// when never updated.
	LastPlain string `json:"lastPlain"`
	// NextPlain is the time until the next scheduled update; empty when no
	// auto-update is scheduled or the update is already due (see Overdue).
	NextPlain string `json:"nextPlain"`
	// Overdue is true when an auto-update is scheduled and its time has come.
	Overdue bool `json:"overdue"`
}

// GetConfigMeta returns display metadata for every profile, keyed by profile
// name. Per-config failures degrade to empty fields instead of failing the
// whole call.
func (b *Bindings) GetConfigMeta() map[string]ConfigMeta {
	recs := b.app.Controller.GetConfigs()
	out := make(map[string]ConfigMeta, len(recs))
	for _, rec := range recs {
		meta := ConfigMeta{}
		if style, err := b.app.Controller.DetectConfigStyle(rec.Name); err != nil {
			b.app.Logger.Root.TDebugf("gui.config_meta_style_failed", rec.Name, err)
		} else if style == inboundstyle.StyleClient || style == inboundstyle.StyleServer {
			meta.Style = string(style)
		}
		if !rec.LastUpdate.IsZero() {
			meta.LastPlain = version.HumanDurationPlain(time.Since(rec.LastUpdate.Time))
		}
		if rec.IsLocal() {
			// A local profile is never downloaded: its "last update" is the
			// config file's mtime, and no scheduled update applies.
			if mt, err := b.app.Controller.LocalConfigModTime(rec.Name); err == nil {
				meta.LastPlain = version.HumanDurationPlain(time.Since(mt))
			}
			meta.NextPlain = ""
			meta.Overdue = false
		} else if next := rec.NextUpdate(); !next.IsZero() {
			if rec.ShouldUpdate() || !time.Now().Before(next) {
				meta.Overdue = true
			} else {
				meta.NextPlain = version.HumanDurationPlain(time.Until(next))
			}
		}
		out[rec.Name] = meta
	}
	return out
}

// SetFallbackType persists the fallback_type for the named profile. When a
// config style check is pending for the profile (the user was asked how to
// treat a non-client config on start), the pending choice is consumed and the
// start flow resumes, mirroring the legacy style dialog. Cancelling that
// dialog simply never calls this binding, so the core is not started.
func (b *Bindings) SetFallbackType(name, fallbackType string) error {
	b.styleCheckMu.Lock()
	choose := b.pendingStyleChecks[name]
	delete(b.pendingStyleChecks, name)
	b.styleCheckMu.Unlock()
	if choose != nil {
		choose(fallbackType)
		return nil
	}
	if err := b.app.Controller.SetFallbackType(name, fallbackType); err != nil {
		b.toastErr(err)
		return err
	}
	b.emitConfigsChanged()
	return nil
}

// UpdateConfigNow downloads the remote config for the named profile using its
// configured URL.
func (b *Bindings) UpdateConfigNow(name string) error {
	rec := b.app.Controller.Config().GetConfigByName(name)
	if rec == nil {
		err := fmt.Errorf("profile not found: %s", name)
		b.toastErr(err)
		return err
	}
	if err := b.app.Controller.UpdateConfigNow(name, rec.URL); err != nil {
		b.toastErr(err)
		return err
	}
	b.emitConfigsChanged()
	b.toastT("success", []string{"configs", "update_now", "done"})
	return nil
}

// UpdateAllConfigs downloads all remote configs. The legacy UI showed a
// progress dialog; the frontend shows a spinner, so only the final result is
// reported.
func (b *Bindings) UpdateAllConfigs() error {
	_, _, err := b.app.Controller.UpdateAllConfigs(nil)
	if err != nil {
		b.toastErr(err)
		return err
	}
	b.emitConfigsChanged()
	b.toastT("success", []string{"configs", "update_all", "done"})
	return nil
}

// ValidateConfig runs a deprecation/validation check for the named profile
// against the installed sing-box core version.
func (b *Bindings) ValidateConfig(name string) (singboxconfig.ValidationResult, error) {
	result, err := b.app.Controller.ValidateConfig(name)
	if err != nil {
		b.toastErr(err)
	}
	return result, err
}

// OpenConfigFile opens the cached config file in the platform default editor.
func (b *Bindings) OpenConfigFile(name string) error {
	if err := b.app.Controller.OpenConfigFile(name); err != nil {
		b.toastErr(err)
		return err
	}
	return nil
}

// OpenConfigDir opens the directory containing the cached config file.
func (b *Bindings) OpenConfigDir(name string) error {
	if err := b.app.Controller.OpenConfigDir(name); err != nil {
		b.toastErr(err)
		return err
	}
	return nil
}

// RecreateLocalConfig recreates the config file of a local profile.
func (b *Bindings) RecreateLocalConfig(name string) error {
	if err := b.app.Controller.RecreateLocalConfig(name); err != nil {
		b.toastErr(err)
		return err
	}
	b.emitConfigsChanged()
	b.toastT("success", []string{"configs", "recreate", "done"})
	return nil
}

// IsConfigHashMismatch reports whether the cached config content differs from
// the hash stored in the profile.
func (b *Bindings) IsConfigHashMismatch(name string) bool {
	return b.app.Controller.IsConfigHashMismatch(name)
}

// HasCachedConfig reports whether a downloaded/created config file exists for
// the named profile.
func (b *Bindings) HasCachedConfig(name string) bool {
	return b.app.Controller.HasCachedConfig(name)
}

// ConfigNameAvailable reports whether a profile name is free to use. exclude
// names the profile being edited, which may keep its own name.
func (b *Bindings) ConfigNameAvailable(name, exclude string) bool {
	return b.app.Controller.ConfigNameAvailable(name, exclude)
}

// GetOrphanedConfigs lists cached config files that no longer have a matching
// profile (leftovers from deleted profiles), with their on-disk mtime.
func (b *Bindings) GetOrphanedConfigs() []core.OrphanedConfig {
	return b.app.Controller.OrphanedConfigs()
}

// DeleteOrphanedConfigs removes the cached files of the named orphan entries
// and reports the result with a toast.
func (b *Bindings) DeleteOrphanedConfigs(names []string) error {
	for _, name := range names {
		if err := b.app.Controller.DeleteCachedConfig(name); err != nil {
			b.toastErr(err)
			return err
		}
	}
	b.emitConfigsChanged()
	b.toastT("success", []string{"configs", "orphans", "deleted"})
	return nil
}

// PickConfigFile opens the native file picker to choose a source file for a
// local profile. It returns the selected path, or an empty string when the
// dialog was cancelled or is unavailable.
func (b *Bindings) PickConfigFile() string {
	app := application.Get()
	if app == nil {
		return ""
	}
	path, err := app.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title:                b.t("configs", "dialog", "pick_file"),
		CanChooseFiles:       true,
		AllowsOtherFileTypes: true,
		Filters: []application.FileFilter{
			{DisplayName: "JSON", Pattern: "*.json"},
		},
	}).PromptForSingleSelection()
	if err != nil {
		return ""
	}
	return path
}
