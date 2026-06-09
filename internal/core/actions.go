package core

import (
	"errors"
	"time"

	"sing-box-ez/internal/config"
)

// PrepareConfig checks that the core exists, validates the active configuration,
// and downloads it if necessary. It returns the active config record or an error.
func PrepareConfig(cfg *config.AppConfig, manager *Manager, logger *Logger) (*config.ConfigRecord, error) {
	if !CoreExists() {
		return nil, errors.New("core not found. Please download it first")
	}

	active := cfg.GetActiveConfig()
	if active == nil {
		return nil, errors.New("no active config. Please add and activate a config in the Configs tab")
	}
	manager.SetConfigURL(active.URL)
	manager.SetConfigName(active.Name)

	if active.ShouldUpdate() || !HasCachedConfig(active.Name) {
		logger.Log("Updating config...")
		if err := manager.UpdateConfig(); err != nil {
			logger.Log("Config download issue: " + err.Error())
			if !HasCachedConfig(active.Name) {
				return nil, errors.New("no config available")
			}
			logger.Log("Using existing config")
		} else {
			cfg.SetLastUpdateFor(active.Name, time.Now())
			_ = cfg.Save()
			logger.Log("Config updated")
		}
	}

	return active, nil
}

// StartCore starts the core process and logs the result.
func StartCore(manager *Manager, logger *Logger) error {
	err := manager.Start()
	if err == nil {
		logger.Log("Sing-box started")
	}
	return err
}

// StopCore stops the core process and logs the result.
func StopCore(manager *Manager, logger *Logger) error {
	err := manager.Stop()
	if err == nil {
		logger.Log("Sing-box stopped")
	}
	return err
}

// RestartCore restarts the core process and logs the result.
func RestartCore(manager *Manager, logger *Logger) error {
	logger.Log("Restarting...")
	err := manager.Restart()
	if err == nil {
		logger.Log("Sing-box restarted")
	}
	return err
}
