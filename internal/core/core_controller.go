package core

import (
	"os"
	"os/exec"
	"runtime"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/framework/logger"
)

// CoreController manages the sing-box core lifecycle and downloads.
type CoreController struct {
	cfg      *config.AppConfig
	manager  *Manager
	logger   *logger.Logger
	terminal *logger.LogTerminal
}

// Terminal returns the logging terminal used by this controller.
func (c *CoreController) Terminal() *logger.LogTerminal {
	return c.terminal
}

// NewCoreController creates a new core lifecycle controller.
func NewCoreController(cfg *config.AppConfig, manager *Manager, logger *logger.Logger, terminal *logger.LogTerminal) *CoreController {
	return &CoreController{
		cfg:      cfg,
		manager:  manager,
		logger:   logger,
		terminal: terminal,
	}
}

// GetInstalledCoreVersion returns the installed core version.
func (c *CoreController) GetInstalledCoreVersion() (string, error) {
	return GetCoreVersion(GetCorePath())
}

// GetLatestCoreVersion returns the latest available core version.
func (c *CoreController) GetLatestCoreVersion() (string, error) {
	return GetLatestVersion()
}

// DownloadCoreWithProgress downloads the latest core and reports progress.
func (c *CoreController) DownloadCoreWithProgress(onProgress func(downloaded, total int64)) (string, error) {
	ver, err := GetLatestVersion()
	if err != nil {
		return "", err
	}
	c.terminal.Info("Latest version: v" + ver)

	path, err := DownloadCore("", onProgress)
	if err != nil {
		return "", err
	}
	c.terminal.Info("Core downloaded to: " + path)
	return path, nil
}

// CoreExists reports whether the core binary is present.
func (c *CoreController) CoreExists() bool {
	return CoreExists()
}

// StartCore starts the core process and logs the result.
func (c *CoreController) StartCore() error {
	_, err := PrepareConfig(c.cfg, c.manager, c.logger)
	if err != nil {
		c.terminal.Error(err.Error())
		return err
	}
	if err := StartCore(c.manager, c.logger); err != nil {
		c.terminal.Error("Failed to start: " + err.Error())
		return err
	}
	c.terminal.Info("Core started")
	return nil
}

// StopCore stops the core process and logs the result.
func (c *CoreController) StopCore() error {
	if err := StopCore(c.manager, c.logger); err != nil {
		c.terminal.Error("Failed to stop: " + err.Error())
		return err
	}
	c.terminal.Info("Core stopped")
	return nil
}

// RestartCore restarts the core process and logs the result.
func (c *CoreController) RestartCore() error {
	c.terminal.Info("Restarting...")
	if err := StopCore(c.manager, c.logger); err != nil {
		c.terminal.Error("Failed to restart: " + err.Error())
		return err
	}
	if err := StartCore(c.manager, c.logger); err != nil {
		c.terminal.Error("Failed to restart: " + err.Error())
		return err
	}
	c.terminal.Info("Core restarted")
	return nil
}

// GetLatestCoreVersionWithLog fetches the latest core version and logs errors.
func (c *CoreController) GetLatestCoreVersionWithLog() (string, error) {
	ver, err := c.GetLatestCoreVersion()
	if err != nil {
		c.terminal.Error("Check failed: " + err.Error())
		return "", err
	}
	return ver, nil
}

// DownloadCoreWithProgressWithLog downloads the core and logs the result.
func (c *CoreController) DownloadCoreWithProgressWithLog(onProgress func(int64, int64)) (string, error) {
	path, err := c.DownloadCoreWithProgress(onProgress)
	if err != nil {
		c.terminal.Error("Failed to download core: " + err.Error())
		return "", err
	}
	return path, nil
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
