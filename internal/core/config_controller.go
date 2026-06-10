package core

import (
	"fmt"
	"time"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/framework/logger"
)

// ConfigController manages configuration records and their lifecycle.
type ConfigController struct {
	cfg      *config.AppConfig
	manager  *Manager
	terminal *logger.LogTerminal
}

// Terminal returns the logging terminal used by this controller.
func (c *ConfigController) Terminal() *logger.LogTerminal {
	return c.terminal
}

// NewConfigController creates a new config controller.
func NewConfigController(cfg *config.AppConfig, manager *Manager, terminal *logger.LogTerminal) *ConfigController {
	return &ConfigController{
		cfg:      cfg,
		manager:  manager,
		terminal: terminal,
	}
}

// HasCachedConfig reports whether a cached config exists for the given name.
func (c *ConfigController) HasCachedConfig(name string) bool {
	return HasCachedConfig(name)
}

// DownloadConfigFor downloads a config by URL and caches it under the given name.
func (c *ConfigController) DownloadConfigFor(name, url string) error {
	return DownloadConfigFor(name, url)
}

// AddFirstConfigWithLog adds the initial config during first-run and logs the result.
func (c *ConfigController) AddFirstConfigWithLog(name, url string) error {
	if url == "" {
		c.terminal.Error("First run: empty config URL")
		return fmt.Errorf("empty config URL")
	}
	rec := config.ConfigRecord{
		Name:                name,
		URL:                 url,
		UpdateIntervalHours: c.cfg.UpdateIntervalHours,
		Parent:              "user",
	}
	c.cfg.AddConfig(rec)
	c.cfg.SetActiveName(name)
	c.cfg.SetFirstRunDone(true)
	_ = c.cfg.Save()
	c.manager.SetConfigURL(url)
	c.manager.SetConfigName(name)
	c.terminal.Info("First config added: " + name)
	return nil
}

// UpdateConfigNowWithLog downloads a config and logs the result.
func (c *ConfigController) UpdateConfigNowWithLog(name, url string) error {
	err := c.DownloadConfigFor(name, url)
	if err != nil {
		c.terminal.Error("Update failed: " + err.Error())
		return err
	}
	c.cfg.SetLastUpdateFor(name, time.Now())
	_ = c.cfg.Save()
	c.terminal.Info("Config updated: " + name)
	return nil
}

// AddConfigWithLog adds a config and logs the result.
func (c *ConfigController) AddConfigWithLog(rec config.ConfigRecord) error {
	if rec.Name == "" || rec.URL == "" {
		c.terminal.Error("Name and URL are required")
		return fmt.Errorf("name and URL are required")
	}
	if c.cfg.GetConfigByName(rec.Name) != nil {
		c.terminal.Error("Config with this name already exists")
		return fmt.Errorf("config already exists")
	}
	c.cfg.AddConfig(rec)
	if c.cfg.GetActiveName() == "" {
		c.cfg.SetActiveName(rec.Name)
		c.manager.SetConfigURL(rec.URL)
		c.manager.SetConfigName(rec.Name)
	}
	_ = c.cfg.Save()
	c.terminal.Info("Config added: " + rec.Name)
	return nil
}

// EditConfigWithLog edits or renames a config and logs the result.
func (c *ConfigController) EditConfigWithLog(oldName string, rec config.ConfigRecord) error {
	if rec.Name == "" || rec.URL == "" {
		c.terminal.Error("Name and URL are required")
		return fmt.Errorf("name and URL are required")
	}
	if rec.Name != oldName {
		if c.cfg.GetConfigByName(rec.Name) != nil {
			c.terminal.Errorf("Config with name %q already exists", rec.Name)
			return fmt.Errorf("config already exists")
		}
		c.cfg.RenameConfig(oldName, rec.Name)
	}
	c.cfg.UpdateConfig(rec.Name, rec)
	_ = c.cfg.Save()
	if oldName == c.cfg.GetActiveName() || rec.Name == c.cfg.GetActiveName() {
		c.manager.SetConfigURL(rec.URL)
	}
	if rec.Name != oldName {
		c.terminal.Info("Config renamed: " + oldName + " -> " + rec.Name)
	} else {
		c.terminal.Info("Config updated: " + rec.Name)
	}
	return nil
}

// DeleteConfigWithLog deletes a config and logs the result.
func (c *ConfigController) DeleteConfigWithLog(name string) error {
	c.cfg.RemoveConfig(name)
	_ = c.cfg.Save()
	c.terminal.Info("Config deleted: " + name)
	return nil
}

// ActivateConfigWithLog activates a cached config and logs the result.
func (c *ConfigController) ActivateConfigWithLog(name string) error {
	rec := c.cfg.GetConfigByName(name)
	if rec == nil {
		return fmt.Errorf("config not found")
	}
	if !c.HasCachedConfig(name) {
		c.terminal.Error("No cached config for: " + name)
		return fmt.Errorf("no cached config")
	}
	c.cfg.SetActiveName(name)
	_ = c.cfg.Save()
	c.manager.SetConfigURL(rec.URL)
	c.manager.SetConfigName(name)
	c.terminal.Info("Activated config: " + name)
	return nil
}

// UpdateAllConfigsWithLog updates all configs and logs progress.
func (c *ConfigController) UpdateAllConfigsWithLog(progress func(done, total int)) (int, int, error) {
	configs := c.cfg.GetConfigs()
	total := 0
	for _, rec := range configs {
		if rec.URL != "" {
			total++
		}
	}
	if total == 0 {
		c.terminal.Info("No configs to update")
		return 0, 0, nil
	}
	updated := 0
	for _, rec := range configs {
		if rec.URL == "" {
			continue
		}
		c.terminal.Info("Updating config: " + rec.Name + "...")
		if err := c.DownloadConfigFor(rec.Name, rec.URL); err != nil {
			c.terminal.Error("Failed to update " + rec.Name + ": " + err.Error())
		} else {
			c.cfg.SetLastUpdateFor(rec.Name, time.Now())
			updated++
			c.terminal.Info("Config updated: " + rec.Name)
		}
		if progress != nil {
			progress(updated, total)
		}
	}
	_ = c.cfg.Save()
	c.terminal.Infof("Update all finished (%d/%d)", updated, total)
	return updated, total, nil
}
