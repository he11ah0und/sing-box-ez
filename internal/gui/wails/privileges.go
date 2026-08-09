//go:build !nogui

package wails

import (
	"fmt"
	"strings"

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
	return nil
}
