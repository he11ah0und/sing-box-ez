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

// NewAppController creates a new app controller.
func NewAppController(cfg *config.AppConfig, log *logger.Logger, parent *logger.LogTerminal, fwApp *framework.App) *AppController {
	return &AppController{cfg: cfg, logger: log, terminal: parent.Allocate("app"), app: fwApp}
}

// Terminal returns the controller's logger terminal.
func (c *AppController) Terminal() *logger.LogTerminal {
	return c.terminal
}

// OpenDataFolder opens the data directory and logs errors.
func (c *AppController) OpenDataFolder() error {
	if c.app == nil {
		return nil
	}
	if err := c.app.OpenDataDir(); err != nil {
		return c.terminal.Errorf("Failed to open data folder: %v", err)
	}
	return nil
}

// SetLogLimit updates the log limit and logs the change.
func (c *AppController) SetLogLimit(v int) {
	c.cfg.SetLogLimit(v)
	_ = c.cfg.Save()
	c.logger.SetLimit(v)
	c.terminal.Infof("Log limit set to %d", v)
}

// SetDefaultInterval updates the default update interval and logs the change.
func (c *AppController) SetDefaultInterval(h int) {
	c.cfg.SetDefaultUpdateInterval(h)
	_ = c.cfg.Save()
	c.terminal.Infof("Default interval set to %dh", h)
}
