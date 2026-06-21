package core

import (
	"runtime"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/framework/localengine"
	"sing-box-ez/internal/framework/logger"
)

// PrivilegeAction describes a single action available in the privilege dialog/tab.
type PrivilegeAction struct {
	ID      string
	Label   string
	Handler func() error
}

// PrivilegeDialog describes the platform-specific privilege elevation dialog.
type PrivilegeDialog struct {
	Title   string
	Message string
	Actions []PrivilegeAction
}

// PrivilegeTabState describes the platform-specific state for the Core privileges tab.
type PrivilegeTabState struct {
	Mode                string // "windows", "linux", "macos"
	IsAdmin             bool
	HasSetcap           bool
	RunAsAdmin          bool
	AdminStatusText     string
	AdminStatusColor    string // "green", "yellow"
	AdminLabel          string
	PrivilegeText       string
	PrivilegeColor      string // "green", "yellow"
	ShowRestartAdminBtn bool
	ShowSetcapBtn       bool
}

// PrivilegeController manages platform-specific privilege operations.
type PrivilegeController struct {
	cfg      *config.AppConfig
	manager  *Manager
	terminal *logger.LogTerminal
}

// NewPrivilegeController creates a new privilege controller.
func NewPrivilegeController(cfg *config.AppConfig, manager *Manager, parent *logger.LogTerminal) *PrivilegeController {
	return &PrivilegeController{
		cfg:      cfg,
		manager:  manager,
		terminal: parent.Allocate("privileges"),
	}
}

// HasRequiredPrivileges reports whether the app has the privileges needed to run the core.
func (c *PrivilegeController) HasRequiredPrivileges() bool {
	switch runtime.GOOS {
	case "linux":
		if HasNetAdminCapability(c.manager.coreBinary()) {
			return true
		}
		return c.cfg.MustGet("privileges", "run_as_admin").Bool()
	case "windows":
		return IsAdmin()
	default:
		return true
	}
}

// RefreshPrivilegeStatus returns the current Linux privilege status.
func (c *PrivilegeController) RefreshPrivilegeStatus() string {
	if runtime.GOOS != "linux" {
		return ""
	}
	if HasNetAdminCapability(c.manager.coreBinary()) {
		return "active"
	}
	return "root_required"
}

// GetPrivilegeDialog returns the dialog definition for the current platform.
func (c *PrivilegeController) GetPrivilegeDialog(restartFn func() error) *PrivilegeDialog {
	switch runtime.GOOS {
	case "darwin":
		return nil
	case "windows":
		return &PrivilegeDialog{
			Title:   localengine.T("dialog", "privileges", "title"),
			Message: localengine.T("dialog", "privileges", "msg_windows"),
			Actions: []PrivilegeAction{
				{
					ID:    "restart_admin",
					Label: localengine.T("dialog", "privileges", "btn_restart_admin"),
					Handler: func() error {
						return restartFn()
					},
				},
			},
		}
	case "linux":
		return &PrivilegeDialog{
			Title:   localengine.T("dialog", "privileges", "title"),
			Message: localengine.T("dialog", "privileges", "msg_linux"),
			Actions: []PrivilegeAction{
				{
					ID:    "setcap",
					Label: localengine.T("dialog", "privileges", "btn_setcap"),
					Handler: func() error {
						return c.ApplySetcap()
					},
				},
				{
					ID:    "run_as_admin",
					Label: localengine.T("dialog", "privileges", "btn_run_as_admin"),
					Handler: func() error {
						c.cfg.MustGet("privileges", "run_as_admin").Update(true)
						c.manager.SetElevated(true)
						return c.cfg.Save()
					},
				},
			},
		}
	default:
		return nil
	}
}

// GetPrivilegeTabState returns the current privilege state for rendering the Core tab.
func (c *PrivilegeController) GetPrivilegeTabState() PrivilegeTabState {
	state := PrivilegeTabState{
		Mode:       runtime.GOOS,
		RunAsAdmin: c.cfg.MustGet("privileges", "run_as_admin").Bool(),
	}

	switch runtime.GOOS {
	case "windows":
		state.IsAdmin = IsAdmin()
		if state.IsAdmin {
			state.AdminStatusText = localengine.T("core", "privileges", "admin")
			state.AdminStatusColor = "green"
		} else {
			state.AdminStatusText = localengine.T("core", "privileges", "user")
			state.AdminStatusColor = "yellow"
		}
		state.ShowRestartAdminBtn = !state.IsAdmin
	case "linux":
		state.HasSetcap = HasNetAdminCapability(c.manager.coreBinary())
		if state.HasSetcap {
			state.AdminLabel = localengine.T("core", "admin", "label_root_setcap")
		} else {
			state.AdminLabel = localengine.T("core", "admin", "label_root_pkexec")
		}
		state.ShowSetcapBtn = true
		status := c.RefreshPrivilegeStatus()
		switch status {
		case "active":
			state.PrivilegeText = localengine.T("core", "privileges", "setcap_active")
			state.PrivilegeColor = "green"
		case "root_required":
			state.PrivilegeText = localengine.T("core", "privileges", "root_required")
			state.PrivilegeColor = "yellow"
		}
	default:
		state.AdminLabel = localengine.T("core", "admin", "label")
	}

	return state
}

// RestartAsAdmin attempts to restart as admin and logs the result.
func (c *PrivilegeController) RestartAsAdmin(restartFn func() error) error {
	if err := restartFn(); err != nil {
		return c.terminal.Errorf("Failed to restart as admin: %v", err)
	}
	return nil
}

// SetRunAsAdmin updates the run-as-admin setting and logs the result.
func (c *PrivilegeController) SetRunAsAdmin(checked bool) error {
	c.cfg.MustGet("privileges", "run_as_admin").Update(checked)
	c.manager.SetElevated(checked)
	if err := c.cfg.Save(); err != nil {
		return c.terminal.Errorf("Failed to save admin setting: %v", err)
	}
	c.terminal.Infof("Admin mode: %v", checked)
	return nil
}

// ApplySetcap applies setcap and logs the result.
func (c *PrivilegeController) ApplySetcap() error {
	if err := SetNetAdminCapabilityGUI(c.manager.coreBinary()); err != nil {
		c.terminal.Errorf("setcap failed: %v", err)
		return c.terminal.Errorf("Tip: run manually: sudo setcap cap_net_admin=+ep ./sing-box")
	}
	c.terminal.Infof("setcap applied successfully.")
	return nil
}

// ApplyPrivilegeAction executes a privilege action, logs the result, and returns
// whether it succeeded, whether the privilege UI should refresh, and whether the app should exit.
func (c *PrivilegeController) ApplyPrivilegeAction(action *PrivilegeAction) (success, needRefresh, needClose bool) {
	err := action.Handler()
	if err != nil {
		c.terminal.Errorf("%s", action.Label+" failed: "+err.Error())
		if action.ID == "setcap" {
			c.terminal.Errorf("Tip: run manually: sudo setcap cap_net_admin=+ep ./sing-box")
		}
		return false, false, false
	}
	c.terminal.Infof("%s", action.Label+" succeeded.")
	needRefresh = action.ID == "setcap" || action.ID == "run_as_admin"
	needClose = action.ID == "restart_admin"
	return true, needRefresh, needClose
}
