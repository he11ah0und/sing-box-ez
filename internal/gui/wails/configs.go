//go:build !nogui

package wails

import (
	"fmt"

	"sing-box-ez/internal/singboxconfig"
)

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
		return fmt.Errorf("profile not found: %s", name)
	}
	if err := b.app.Controller.UpdateConfigNow(name, rec.URL); err != nil {
		return err
	}
	b.emitConfigsChanged()
	return nil
}

// UpdateAllConfigs downloads all remote configs. The legacy UI showed a
// progress dialog; the frontend shows a spinner, so only the final result is
// reported.
func (b *Bindings) UpdateAllConfigs() error {
	_, _, err := b.app.Controller.UpdateAllConfigs(nil)
	if err != nil {
		return err
	}
	b.emitConfigsChanged()
	return nil
}

// ValidateConfig runs a deprecation/validation check for the named profile
// against the installed sing-box core version.
func (b *Bindings) ValidateConfig(name string) (singboxconfig.ValidationResult, error) {
	return b.app.Controller.ValidateConfig(name)
}

// OpenConfigFile opens the cached config file in the platform default editor.
func (b *Bindings) OpenConfigFile(name string) error {
	return b.app.Controller.OpenConfigFile(name)
}

// OpenConfigDir opens the directory containing the cached config file.
func (b *Bindings) OpenConfigDir(name string) error {
	return b.app.Controller.OpenConfigDir(name)
}

// RecreateLocalConfig recreates the config file of a local profile.
func (b *Bindings) RecreateLocalConfig(name string) error {
	if err := b.app.Controller.RecreateLocalConfig(name); err != nil {
		return err
	}
	b.emitConfigsChanged()
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
