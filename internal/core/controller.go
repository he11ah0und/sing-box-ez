package core

import (
	"sync"

	"sing-box-ez/internal/config"
)

// Controller is the base entry-point for core business logic.
// It provides lifecycle management, logging, and config operations.
// Interactive UI layers should use InteractiveController instead.
type Controller struct {
	cfg     *config.AppConfig
	manager *Manager
	logger  *Logger

	stopped bool
	stopMu  sync.Mutex
}

// NewController creates a new base controller with manager and logger.
func NewController(cfg *config.AppConfig) *Controller {
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
	logger := NewLogger(cfg, logWriter, manager)
	logger.Start()

	c := &Controller{
		cfg:     cfg,
		manager: manager,
		logger:  logger,
	}

	return c
}

// Close shuts down the controller: stops logging and the core if running.
func (c *Controller) Close() {
	c.stopMu.Lock()
	c.stopped = true
	c.stopMu.Unlock()
	if c.logger != nil {
		c.logger.Close()
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

// ---------- Logging ----------

func (c *Controller) Log(msg string) {
	c.logger.Log(msg)
}

func (c *Controller) GetLogLines() []string {
	return c.logger.GetLines()
}

func (c *Controller) ClearLogs() {
	c.logger.Clear()
}

// Logger returns the internal logger for advanced use (e.g. setting callbacks).
func (c *Controller) Logger() *Logger {
	return c.logger
}

// ---------- Core lifecycle ----------

func (c *Controller) PrepareConfig() (*config.ConfigRecord, error) {
	return PrepareConfig(c.cfg, c.manager, c.logger)
}

func (c *Controller) Start() error {
	return StartCore(c.manager, c.logger)
}

func (c *Controller) Stop() error {
	return StopCore(c.manager, c.logger)
}

func (c *Controller) Restart() error {
	return RestartCore(c.manager, c.logger)
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
