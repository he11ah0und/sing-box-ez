//go:build !nogui

package wails

import (
	"errors"
	"fmt"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/he11ah0und/projectspec"
	"sing-box-ez/internal/core"
)

// PrivilegeTabState is the UI-facing snapshot of core.PrivilegeTabState for
// the privileges section of the Core page.
type PrivilegeTabState struct {
	Mode                string `json:"mode"`
	IsAdmin             bool   `json:"isAdmin"`
	HasSetcap           bool   `json:"hasSetcap"`
	AdminStatusText     string `json:"adminStatusText"`
	AdminStatusColor    string `json:"adminStatusColor"`
	AdminLabel          string `json:"adminLabel"`
	PrivilegeText       string `json:"privilegeText"`
	PrivilegeColor      string `json:"privilegeColor"`
	ShowRestartAdminBtn bool   `json:"showRestartAdminBtn"`
	ShowSetcapBtn       bool   `json:"showSetcapBtn"`
}

// GetPrivilegeTabState returns the current platform-specific privilege state.
func (b *Bindings) GetPrivilegeTabState() PrivilegeTabState {
	s := b.app.Controller.GetPrivilegeTabState()
	return PrivilegeTabState{
		Mode:                s.Mode,
		IsAdmin:             s.IsAdmin,
		HasSetcap:           s.HasSetcap,
		AdminStatusText:     s.AdminStatusText,
		AdminStatusColor:    s.AdminStatusColor,
		AdminLabel:          s.AdminLabel,
		PrivilegeText:       s.PrivilegeText,
		PrivilegeColor:      s.PrivilegeColor,
		ShowRestartAdminBtn: s.ShowRestartAdminBtn,
		ShowSetcapBtn:       s.ShowSetcapBtn,
	}
}

// RestartAsAdmin restarts the application with administrator privileges.
// It is a no-op on non-Windows platforms.
func (b *Bindings) RestartAsAdmin() error {
	if err := b.app.Controller.RestartAsAdmin(); err != nil {
		b.toastErr(err, "settings", "privileges", "restart_admin")
		return err
	}
	return nil
}

// ApplySetcap applies the cap_net_admin capability to the sing-box core binary
// so TUN mode works without root (Linux).
func (b *Bindings) ApplySetcap() error {
	if err := b.app.Controller.ApplySetcap(); err != nil {
		b.toastErr(err, "settings", "privileges", "setcap")
		return err
	}
	return nil
}

// configActionEntry resolves a "control: action" config entry by its
// dot-joined path (e.g. "privileges.setcap").
func (b *Bindings) configActionEntry(path string) (*projectspec.ConfigEntry, error) {
	spec := b.app.Spec
	if spec == nil {
		return nil, fmt.Errorf("no project spec loaded")
	}
	for i := range spec.Config {
		e := &spec.Config[i]
		if strings.Join(e.Path, ".") != path {
			continue
		}
		if e.Control != "action" || e.Action == "" {
			return nil, fmt.Errorf("config path %q is not an action entry", path)
		}
		return e, nil
	}
	return nil, fmt.Errorf("unknown config path %q", path)
}

// RunConfigAction runs the backend action declared by a "control: action"
// config entry (see internal/app/project.yaml), addressed by its dot-joined
// path. Outcomes are logged once through ApplyPrivilegeAction.
func (b *Bindings) RunConfigAction(path string) error {
	entry, err := b.configActionEntry(path)
	if err != nil {
		b.toastErr(err)
		return err
	}
	// The core-binary picker is not a privilege action: it opens a file
	// dialog, validates compatibility and stores the chosen path.
	if entry.Action == "browse_core_binary" {
		return b.browseCoreBinary()
	}
	var handler func() error
	switch entry.Action {
	case "setcap":
		// Toggle: remove the capability when present, apply it when missing.
		handler = b.app.Controller.ToggleSetcap
	case "restart_admin":
		handler = b.app.Controller.RestartAsAdmin
	default:
		err := fmt.Errorf("unknown config action %q for %q", entry.Action, path)
		b.toastErr(err)
		return err
	}
	success, _, _ := b.app.Controller.ApplyPrivilegeAction(&core.PrivilegeAction{
		ID:      entry.Action,
		Handler: handler,
	})
	if !success {
		// ApplyPrivilegeAction already logged the failure; surface a plain
		// error so the frontend can notify the user.
		err := fmt.Errorf("config action %q failed", entry.Action)
		b.toastErr(err)
		return err
	}
	// The setcap toggle reads back the resulting capability state to pick the
	// right success message; restart_admin exits the app and needs no toast.
	if entry.Action == "setcap" {
		if b.app.Controller.GetPrivilegeTabState().HasSetcap {
			b.toastT("success", []string{"settings", "privileges", "setcap_done"})
		} else {
			b.toastT("success", []string{"settings", "privileges", "setcap_removed"})
		}
	}
	return nil
}

// browseCoreBinary opens a file dialog for the custom core binary path,
// verifies the picked binary against MinCoreVersion and stores the path.
func (b *Bindings) browseCoreBinary() error {
	app := application.Get()
	if app == nil {
		return nil
	}
	path, err := app.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title:                b.t("settings", "core", "source", "browse_custom_path"),
		CanChooseFiles:       true,
		AllowsOtherFileTypes: true,
	}).PromptForSingleSelection()
	if err != nil || path == "" {
		// Cancelled dialogs are not an error.
		return nil
	}
	if _, err := core.CheckCoreCompatibility(path); err != nil {
		b.toastErr(err)
		return err
	}
	cfg := b.app.Controller.Config()
	if err := cfg.MustGet("core", "source", "custom_path").Update(path); err != nil {
		b.toastErr(err)
		return err
	}
	_ = cfg.Save()
	b.emit("settings:changed", map[string]any{"core.source.custom_path": path})
	return nil
}

// SetCoreSourceMode switches the core source mode (official/system/custom)
// from the updates tab; the choice is persisted in core.source.mode. system
// requires a compatible system-installed sing-box; custom opens the file
// picker when no path is stored yet.
func (b *Bindings) SetCoreSourceMode(mode string) error {
	cfg := b.app.Controller.Config()
	switch mode {
	case core.CoreSourceOfficial:
	case core.CoreSourceSystem:
		bin := core.FindSystemCore()
		if bin == "" {
			err := errors.New(b.t("settings", "core", "source", "system_not_found"))
			b.toast("error", err.Error(), "")
			return err
		}
		if _, err := core.CheckCoreCompatibility(bin); err != nil {
			b.toastErr(err)
			return err
		}
	case core.CoreSourceCustom:
		path := cfg.MustGet("core", "source", "custom_path").String()
		if path == "" {
			if err := b.browseCoreBinary(); err != nil {
				return err
			}
			path = cfg.MustGet("core", "source", "custom_path").String()
			if path == "" {
				// Picker cancelled; keep the current mode.
				return nil
			}
		}
		if _, err := core.CheckCoreCompatibility(path); err != nil {
			b.toastErr(err)
			return err
		}
	default:
		err := fmt.Errorf("unknown core source mode %q", mode)
		b.toastErr(err)
		return err
	}
	if err := cfg.MustGet("core", "source", "mode").Update(mode); err != nil {
		b.toastErr(err)
		return err
	}
	_ = cfg.Save()
	b.emit("settings:changed", map[string]any{"core.source.mode": mode})
	// The mode change swaps the core binary; setcap state follows it.
	b.emit("privileges:changed", struct{}{})
	return nil
}
