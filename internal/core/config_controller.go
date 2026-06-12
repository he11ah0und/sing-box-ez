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

// NewConfigController creates a new config controller.
func NewConfigController(cfg *config.AppConfig, manager *Manager, parent *logger.LogTerminal) *ConfigController {
	return &ConfigController{
		cfg:      cfg,
		manager:  manager,
		terminal: parent.Allocate("config"),
	}
}

// Terminal returns the controller's logger terminal.
func (c *ConfigController) Terminal() *logger.LogTerminal {
	return c.terminal
}

// HasCachedConfig reports whether a cached config exists for the given name.
func (c *ConfigController) HasCachedConfig(name string) bool {
	return HasCachedConfig(name)
}

// DownloadConfigFor downloads a config by URL and caches it under the given name.
func (c *ConfigController) DownloadConfigFor(name, url string) error {
	return DownloadConfigFor(name, url)
}

// AddFirstConfig adds the initial config during first-run and logs the result.
func (c *ConfigController) AddFirstConfig(name, url string) error {
	if url == "" {
		return c.terminal.Errorf("First run: empty config URL")
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
	c.terminal.Infof("First config added: " + name)
	return nil
}

// UpdateConfigNow downloads a config and logs the result.
func (c *ConfigController) UpdateConfigNow(name, url string) error {
	if err := c.DownloadConfigFor(name, url); err != nil {
		return c.terminal.Errorf("Update failed: %v", err)
	}
	c.cfg.SetLastUpdateFor(name, time.Now())
	_ = c.cfg.Save()
	c.terminal.Infof("Config updated: " + name)
	return nil
}

// AddConfig adds a config and logs the result.
func (c *ConfigController) AddConfig(rec config.ConfigRecord) error {
	if rec.Name == "" || rec.URL == "" {
		return c.terminal.Errorf("Name and URL are required")
	}
	if c.cfg.GetConfigByName(rec.Name) != nil {
		return c.terminal.Errorf("Config with this name already exists")
	}
	c.cfg.AddConfig(rec)
	if c.cfg.GetActiveName() == "" {
		c.cfg.SetActiveName(rec.Name)
		c.manager.SetConfigURL(rec.URL)
		c.manager.SetConfigName(rec.Name)
	}
	_ = c.cfg.Save()
	c.terminal.Infof("Config added: " + rec.Name)
	return nil
}

// EditConfig edits or renames a config and logs the result.
func (c *ConfigController) EditConfig(oldName string, rec config.ConfigRecord) error {
	if rec.Name == "" || rec.URL == "" {
		return c.terminal.Errorf("Name and URL are required")
	}
	if rec.Name != oldName {
		if c.cfg.GetConfigByName(rec.Name) != nil {
			return c.terminal.Errorf("Config with name %q already exists", rec.Name)
		}
		c.cfg.RenameConfig(oldName, rec.Name)
	}
	c.cfg.UpdateConfig(rec.Name, rec)
	_ = c.cfg.Save()
	if oldName == c.cfg.GetActiveName() || rec.Name == c.cfg.GetActiveName() {
		c.manager.SetConfigURL(rec.URL)
	}
	if rec.Name != oldName {
		c.terminal.Infof("Config renamed: " + oldName + " -> " + rec.Name)
	} else {
		c.terminal.Infof("Config updated: " + rec.Name)
	}
	return nil
}

// DeleteConfig deletes a config and logs the result.
func (c *ConfigController) DeleteConfig(name string) error {
	c.cfg.RemoveConfig(name)
	_ = c.cfg.Save()
	c.terminal.Infof("Config deleted: " + name)
	return nil
}

// ActivateConfig activates a cached config and logs the result.
func (c *ConfigController) ActivateConfig(name string) error {
	rec := c.cfg.GetConfigByName(name)
	if rec == nil {
		return fmt.Errorf("config not found")
	}
	if !c.HasCachedConfig(name) {
		return c.terminal.Errorf("No cached config for: %s", name)
	}
	c.cfg.SetActiveName(name)
	_ = c.cfg.Save()
	c.manager.SetConfigURL(rec.URL)
	c.manager.SetConfigName(name)
	c.terminal.Infof("Activated config: " + name)
	return nil
}

// UpdateAllConfigs updates all configs and logs progress.
func (c *ConfigController) UpdateAllConfigs(progress func(done, total int)) (int, int, error) {
	configs := c.cfg.GetConfigs()
	total := 0
	for _, rec := range configs {
		if rec.URL != "" {
			total++
		}
	}
	if total == 0 {
		c.terminal.Infof("No configs to update")
		return 0, 0, nil
	}
	updated := 0
	for _, rec := range configs {
		if rec.URL == "" {
			continue
		}
		c.terminal.Infof("Updating config: " + rec.Name + "...")
		if err := c.DownloadConfigFor(rec.Name, rec.URL); err != nil {
			c.terminal.Errorf("Failed to update %s: %v", rec.Name, err)
		} else {
			c.cfg.SetLastUpdateFor(rec.Name, time.Now())
			updated++
			c.terminal.Infof("Config updated: " + rec.Name)
		}
		if progress != nil {
			progress(updated, total)
		}
	}
	_ = c.cfg.Save()
	c.terminal.Infof("Update all finished (%d/%d)", updated, total)
	return updated, total, nil
}
