package core

import (
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/util/paths"
)

// AppController manages general application-level actions and settings.
type AppController struct {
	cfg      *config.AppConfig
	terminal *LogTerminal
}

// Terminal returns the logging terminal used by this controller.
func (c *AppController) Terminal() *LogTerminal {
	return c.terminal
}

// NewAppController creates a new app controller.
func NewAppController(cfg *config.AppConfig, terminal *LogTerminal) *AppController {
	return &AppController{cfg: cfg, terminal: terminal}
}

// OpenDataFolderWithLog opens the data directory and logs errors.
func (c *AppController) OpenDataFolderWithLog() error {
	err := paths.OpenDataDir()
	if err != nil {
		c.terminal.Error("Failed to open data folder: " + err.Error())
		return err
	}
	return nil
}

// SetLogLimitWithLog updates the log limit and logs the change.
func (c *AppController) SetLogLimitWithLog(v int) {
	c.cfg.SetLogLimit(v)
	_ = c.cfg.Save()
	c.terminal.Infof("Log limit set to %d", v)
}

// SetDefaultIntervalWithLog updates the default update interval and logs the change.
func (c *AppController) SetDefaultIntervalWithLog(h int) {
	c.cfg.SetDefaultUpdateInterval(h)
	_ = c.cfg.Save()
	c.terminal.Infof("Default interval set to %dh", h)
}
