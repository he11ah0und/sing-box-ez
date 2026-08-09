package core

import (
	"runtime"

	"github.com/he11ah0und/localengine"
	"github.com/he11ah0und/logger"
	"sing-box-ez/internal/config"
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
		return HasNetAdminCapability(c.manager.coreBinary())
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
					Label: localengine.T("core", "btn", "restart_admin"),
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
			},
		}
	default:
		return nil
	}
}

// GetPrivilegeTabState returns the current privilege state for rendering the Core tab.
func (c *PrivilegeController) GetPrivilegeTabState() PrivilegeTabState {
	state := PrivilegeTabState{
		Mode: runtime.GOOS,
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
		state.ShowSetcapBtn = !state.HasSetcap
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

// RestartAsAdmin attempts to restart as admin and returns any error.
func (c *PrivilegeController) RestartAsAdmin(restartFn func() error) error {
	return restartFn()
}

// ApplySetcap applies setcap and returns any error.
func (c *PrivilegeController) ApplySetcap() error {
	return SetNetAdminCapabilityGUI(c.manager.coreBinary())
}

// ApplyPrivilegeAction executes a privilege action, logs the result, and returns
// whether it succeeded, whether the privilege UI should refresh, and whether the app should exit.
func (c *PrivilegeController) ApplyPrivilegeAction(action *PrivilegeAction) (success, needRefresh, needClose bool) {
	err := action.Handler()
	if err != nil {
		_ = c.terminal.TErrorf("core.privileges.action_failed", action.ID, err)
		if action.ID == "setcap" {
			_ = c.terminal.TErrorf("core.privileges.setcap_tip")
		}
		return false, false, false
	}
	c.terminal.TInfof("core.privileges.action_succeeded", action.ID)
	needRefresh = action.ID == "setcap"
	needClose = action.ID == "restart_admin"
	return true, needRefresh, needClose
}
