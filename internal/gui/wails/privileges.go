//go:build !nogui

package wails

// PrivilegeTabState is the UI-facing snapshot of core.PrivilegeTabState for
// the privileges section of the Core page.
type PrivilegeTabState struct {
	Mode                string `json:"mode"`
	IsAdmin             bool   `json:"isAdmin"`
	HasSetcap           bool   `json:"hasSetcap"`
	RunAsAdmin          bool   `json:"runAsAdmin"`
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
		RunAsAdmin:          s.RunAsAdmin,
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
	return b.app.Controller.RestartAsAdmin()
}

// ApplySetcap applies the cap_net_admin capability to the sing-box core binary
// so TUN mode works without root (Linux).
func (b *Bindings) ApplySetcap() error {
	return b.app.Controller.ApplySetcap()
}
