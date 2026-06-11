package core

import (
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/framework"
	"sing-box-ez/internal/framework/logger"
)

// AppController manages general application-level actions and settings.
type AppController struct {
	cfg      *config.AppConfig
	logger   *logger.Logger
	terminal *logger.LogTerminal
	app      *framework.App
}

// Terminal returns the logging terminal used by this controller.
func (c *AppController) Terminal() *logger.LogTerminal {
	return c.terminal
}

// NewAppController creates a new app controller.
func NewAppController(cfg *config.AppConfig, log *logger.Logger, terminal *logger.LogTerminal, fwApp *framework.App) *AppController {
	return &AppController{cfg: cfg, logger: log, terminal: terminal, app: fwApp}
}

// OpenDataFolderWithLog opens the data directory and logs errors.
func (c *AppController) OpenDataFolderWithLog() error {
	if c.app == nil {
		return nil
	}
	err := c.app.OpenDataDir()
	if err != nil {
		c.terminal.Errorf("Failed to open data folder: " + err.Error())
		return err
	}
	return nil
}

// SetLogLimitWithLog updates the log limit and logs the change.
func (c *AppController) SetLogLimitWithLog(v int) {
	c.cfg.SetLogLimit(v)
	_ = c.cfg.Save()
	c.logger.SetLimit(v)
	c.terminal.Infof("Log limit set to %d", v)
}

// SetDefaultIntervalWithLog updates the default update interval and logs the change.
func (c *AppController) SetDefaultIntervalWithLog(h int) {
	c.cfg.SetDefaultUpdateInterval(h)
	_ = c.cfg.Save()
	c.terminal.Infof("Default interval set to %dh", h)
}
