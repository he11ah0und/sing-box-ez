package core

import (
	"context"
	"os"
	"os/exec"
	"runtime"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/framework/logger"
	"sing-box-ez/internal/framework/updater"
)

// CoreController manages the sing-box core lifecycle and downloads.
type CoreController struct {
	cfg      *config.AppConfig
	manager  *Manager
	logger   *logger.Logger
	terminal *logger.LogTerminal
}

// NewCoreController creates a new core lifecycle controller.
func NewCoreController(cfg *config.AppConfig, manager *Manager, logger *logger.Logger, parent *logger.LogTerminal) *CoreController {
	return &CoreController{
		cfg:      cfg,
		manager:  manager,
		logger:   logger,
		terminal: parent.Allocate("core"),
	}
}

// Terminal returns the controller's logger terminal.
func (c *CoreController) Terminal() *logger.LogTerminal {
	return c.terminal
}

// GetInstalledCoreVersion returns the installed core version.
func (c *CoreController) GetInstalledCoreVersion() (string, error) {
	return GetCoreVersion(GetCorePath())
}

// GetLatestCoreVersion returns the latest available core version and logs errors.
func (c *CoreController) GetLatestCoreVersion() (string, error) {
	m := coreUpdater()
	if m == nil {
		return "", c.terminal.Errorf("core updater not configured")
	}
	info, err := m.Check(context.Background(), "")
	if err != nil {
		return "", err
	}
	return info.Latest, nil
}

// DownloadCoreWithProgress downloads the latest core, reports progress and logs the result.
func (c *CoreController) DownloadCoreWithProgress(onProgress func(downloaded, total int64)) (string, error) {
	m := coreUpdater()
	if m == nil {
		return "", c.terminal.Errorf("core updater not configured")
	}

	info, err := m.Check(context.Background(), "")
	if err != nil {
		return "", err
	}
	if info.ReleaseCount == 0 {
		c.terminal.Infof("Core is up to date: %s", info.Current)
		return GetCorePath(), nil
	}

	c.terminal.Infof("Latest core version: %s", info.Latest)
	info.Files = []updater.UpdateFile{{
		Asset:    info.Asset,
		DestPath: ".",
	}}
	if err := m.Install(context.Background(), info, onProgress); err != nil {
		return "", c.terminal.Errorf("Failed to download core: %v", err)
	}
	c.terminal.Infof("Core downloaded to: %s", GetCorePath())
	return GetCorePath(), nil
}

// CoreExists reports whether the core binary is present.
func (c *CoreController) CoreExists() bool {
	return CoreExists()
}

// StartCore starts the core process and logs the result.
func (c *CoreController) StartCore() error {
	_, err := PrepareConfig(c.cfg, c.manager, c.logger)
	if err != nil {
		return c.terminal.Errorf("Failed to start: %v", err)
	}
	if err := StartCore(c.manager, c.logger); err != nil {
		return c.terminal.Errorf("Failed to start: %v", err)
	}
	c.terminal.Infof("Core started")
	return nil
}

// StopCore stops the core process and logs the result.
func (c *CoreController) StopCore() error {
	if err := StopCore(c.manager, c.logger); err != nil {
		return c.terminal.Errorf("Failed to stop: %v", err)
	}
	c.terminal.Infof("Core stopped")
	return nil
}

// RestartCore restarts the core process and logs the result.
func (c *CoreController) RestartCore() error {
	c.terminal.Infof("Restarting...")
	if err := StopCore(c.manager, c.logger); err != nil {
		return c.terminal.Errorf("Failed to restart: %v", err)
	}
	if err := StartCore(c.manager, c.logger); err != nil {
		return c.terminal.Errorf("Failed to restart: %v", err)
	}
	c.terminal.Infof("Core restarted")
	return nil
}

// RestartAsAdmin restarts the application with elevated privileges (Windows only).
func (c *CoreController) RestartAsAdmin() error {
	if runtime.GOOS != "windows" {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	// #nosec G204 — powershell is a system binary; exe comes from os.Executable() and cwd from os.Getwd().
	cmd := exec.Command("powershell", "-WindowStyle", "hidden", "-Command",
		"Start-Process", "-FilePath", exe, "-Verb", "runAs", "-WorkingDirectory", cwd)
	return cmd.Start()
}

func coreUpdater() *updater.Manager {
	return CoreUpdater
}
