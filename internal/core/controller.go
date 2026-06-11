package core

import (
	"sync"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/framework"
	"sing-box-ez/internal/framework/logger"
)

// Controller is the base entry-point for core business logic.
// It provides lifecycle management, logging, and config operations.
// Interactive UI layers should use InteractiveController instead.
type Controller struct {
	cfg       *config.AppConfig
	manager   *Manager
	app       *framework.App
	processor *CoreLogProcessor

	stopped bool
	stopMu  sync.Mutex
}

// NewController creates a new base controller with manager and logger.
func NewController(cfg *config.AppConfig, fwApp *framework.App) *Controller {
	active := cfg.GetActiveConfig()
	url := ""
	if active != nil {
		url = active.URL
	}
	manager := NewManager(url)
	if active != nil {
		manager.SetConfigName(active.Name)
	}
	manager.SetElevated(cfg.RunAsAdmin)

	logWriter := NewCoreLogWriter()
	manager.SetLogOutput(logWriter)

	log := fwApp.Logger
	processor := NewCoreLogProcessor(cfg, manager, logWriter, log.Root())
	processor.Start()

	c := &Controller{
		cfg:       cfg,
		manager:   manager,
		app:       fwApp,
		processor: processor,
	}

	return c
}

// Close shuts down the controller: stops logging and the core if running.
func (c *Controller) Close() {
	c.stopMu.Lock()
	c.stopped = true
	c.stopMu.Unlock()
	if c.processor != nil {
		c.processor.Stop()
	}
	if c.manager.IsRunning() {
		_ = c.manager.Stop()
	}
}

func (c *Controller) isStopped() bool {
	c.stopMu.Lock()
	defer c.stopMu.Unlock()
	return c.stopped
}

// ---------- Config pass-through ----------

func (c *Controller) Config() *config.AppConfig {
	return c.cfg
}

// ---------- Framework app ----------

// Framework returns the framework App containing cross-cutting services.
func (c *Controller) Framework() *framework.App {
	return c.app
}

// OpenDataDir opens the app's base directory in the system file manager.
func (c *Controller) OpenDataDir() error {
	if c.app == nil {
		return nil
	}
	return c.app.OpenDataDir()
}

// ---------- Logging ----------

func (c *Controller) Log(msg string) {
	c.app.Logger.Log(msg)
}

func (c *Controller) LogRoot() *logger.LogTerminal {
	return c.app.Logger.Root()
}

func (c *Controller) GetLogLines() []string {
	return c.app.Logger.GetLines()
}

func (c *Controller) ClearLogs() {
	c.app.Logger.Clear()
}

// Logger returns the internal logger for advanced use.
func (c *Controller) Logger() *logger.Logger {
	return c.app.Logger
}

// LogProcessor returns the core log processor (for setting callbacks such as OnAutoRestart).
func (c *Controller) LogProcessor() *CoreLogProcessor {
	return c.processor
}

// Manager returns the internal process manager.
func (c *Controller) Manager() *Manager {
	return c.manager
}

// ---------- Core lifecycle ----------

func (c *Controller) PrepareConfig() (*config.ConfigRecord, error) {
	return PrepareConfig(c.cfg, c.manager, c.app.Logger)
}

func (c *Controller) Start() error {
	return StartCore(c.manager, c.app.Logger)
}

func (c *Controller) Stop() error {
	return StopCore(c.manager, c.app.Logger)
}

func (c *Controller) Restart() error {
	return RestartCore(c.manager, c.app.Logger)
}

func (c *Controller) IsRunning() bool {
	return c.manager.IsRunning()
}

func (c *Controller) GetPID() int {
	return c.manager.GetPID()
}

func (c *Controller) SetConfigURL(url string) {
	c.manager.SetConfigURL(url)
}

func (c *Controller) SetConfigName(name string) {
	c.manager.SetConfigName(name)
}

func (c *Controller) SetElevated(v bool) {
	c.manager.SetElevated(v)
}

func (c *Controller) UpdateConfig() error {
	return c.manager.UpdateConfig()
}
