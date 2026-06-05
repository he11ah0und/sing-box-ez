package config

import (
	"encoding/json"
	"os"
	"sing-box-ez/internal/paths"
	"sync"
	"time"
)

// ConfigRecord represents a single subscription / config entry.
type ConfigRecord struct {
	Name                string    `json:"name"`
	URL                 string    `json:"url"`
	UpdateIntervalHours int       `json:"update_interval_hours"`
	LastUpdate          time.Time `json:"last_update"`
	// Parent identifies who created this config: "user" for user-created,
	// or "pl-{plugin_id}" for plugin-created configs.
	Parent string `json:"parent"`
}

// NextUpdate returns the next scheduled update time for this record.
func (r *ConfigRecord) NextUpdate() time.Time {
	if r.LastUpdate.IsZero() {
		return time.Time{}
	}
	return r.LastUpdate.Add(time.Duration(r.UpdateIntervalHours) * time.Hour)
}

// ShouldUpdate reports whether this config is due for an update.
func (r *ConfigRecord) ShouldUpdate() bool {
	if r.LastUpdate.IsZero() {
		return true
	}
	return time.Since(r.LastUpdate) > time.Duration(r.UpdateIntervalHours)*time.Hour
}

type AppConfig struct {
	// Legacy single-config fields kept for backwards compatibility
	SingBoxURL          string `json:"singbox_url,omitempty"`
	UpdateIntervalHours int    `json:"update_interval_hours"`

	RunAsAdmin       bool `json:"run_as_admin"`
	ShowLogs         bool `json:"show_logs"`
	ShowCoreLogs     bool `json:"show_core_logs"`
	LogLimit         int  `json:"log_limit"`
	PluginsEnabled   bool `json:"plugins_enabled"`
	PluginsDeveloper bool `json:"plugins_developer"`
	CoreAutoRestart  bool `json:"core_auto_restart"`
	FirstRunDone     bool `json:"first_run_done"`

	// New multi-config list
	Configs    []ConfigRecord `json:"configs"`
	ActiveName string         `json:"active_name"`

	mu sync.RWMutex
}

func Load() (*AppConfig, error) {
	data, err := os.ReadFile(paths.AppConfig())
	if err != nil {
		if os.IsNotExist(err) {
			cfg := &AppConfig{
				UpdateIntervalHours: 2,
				RunAsAdmin:          false,
				ShowLogs:            true,
				ShowCoreLogs:        false,
				LogLimit:            100,
				CoreAutoRestart:     true,
				Configs:             []ConfigRecord{},
			}
			return cfg, cfg.Save()
		}
		return nil, err
	}
	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	// Migrate legacy single URL into the new list if needed.
	if cfg.SingBoxURL != "" && len(cfg.Configs) == 0 {
		cfg.Configs = append(cfg.Configs, ConfigRecord{
			Name:                "default",
			URL:                 cfg.SingBoxURL,
			UpdateIntervalHours: cfg.UpdateIntervalHours,
			Parent:              "user",
		})
		cfg.ActiveName = "default"
		cfg.SingBoxURL = ""
		_ = cfg.Save()
	}
	return &cfg, nil
}

func (c *AppConfig) Save() error {
	c.mu.RLock()
	data, err := json.MarshalIndent(c, "", "  ")
	c.mu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(paths.AppConfig(), data, 0600)
}

func (c *AppConfig) GetConfigs() []ConfigRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]ConfigRecord, len(c.Configs))
	copy(out, c.Configs)
	return out
}

func (c *AppConfig) GetActiveConfig() *ConfigRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.ActiveName == "" && len(c.Configs) > 0 {
		return &c.Configs[0]
	}
	for i := range c.Configs {
		if c.Configs[i].Name == c.ActiveName {
			return &c.Configs[i]
		}
	}
	return nil
}

func (c *AppConfig) GetConfigByName(name string) *ConfigRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for i := range c.Configs {
		if c.Configs[i].Name == name {
			return &c.Configs[i]
		}
	}
	return nil
}

// GetConfigsByParent returns all configs created by the given parent.
func (c *AppConfig) GetConfigsByParent(parent string) []ConfigRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var out []ConfigRecord
	for i := range c.Configs {
		if c.Configs[i].Parent == parent {
			out = append(out, c.Configs[i])
		}
	}
	return out
}

// GetConfigByNameAndParent returns a config only if it matches both name and parent.
func (c *AppConfig) GetConfigByNameAndParent(name, parent string) *ConfigRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for i := range c.Configs {
		if c.Configs[i].Name == name && c.Configs[i].Parent == parent {
			return &c.Configs[i]
		}
	}
	return nil
}

func (c *AppConfig) SetActiveName(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ActiveName = name
}

func (c *AppConfig) AddConfig(rec ConfigRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Configs = append(c.Configs, rec)
}

func (c *AppConfig) UpdateConfig(name string, rec ConfigRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.Configs {
		if c.Configs[i].Name == name {
			c.Configs[i] = rec
			return
		}
	}
}

func (c *AppConfig) RemoveConfig(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.Configs {
		if c.Configs[i].Name == name {
			c.Configs = append(c.Configs[:i], c.Configs[i+1:]...)
			if c.ActiveName == name {
				c.ActiveName = ""
			}
			return
		}
	}
}

func (c *AppConfig) SetLastUpdateFor(name string, t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.Configs {
		if c.Configs[i].Name == name {
			c.Configs[i].LastUpdate = t
			return
		}
	}
}

func (c *AppConfig) SetDefaultUpdateInterval(hours int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.UpdateIntervalHours = hours
}

func (c *AppConfig) RenameConfig(oldName, newName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.Configs {
		if c.Configs[i].Name == oldName {
			c.Configs[i].Name = newName
			break
		}
	}
	if c.ActiveName == oldName {
		c.ActiveName = newName
	}
}

// Legacy helpers
func (c *AppConfig) SetURL(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.SingBoxURL = url
}

func (c *AppConfig) SetRunAsAdmin(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.RunAsAdmin = v
}

func (c *AppConfig) SetShowLogs(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ShowLogs = v
}

func (c *AppConfig) GetShowLogs() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ShowLogs
}

func (c *AppConfig) SetShowCoreLogs(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ShowCoreLogs = v
}

func (c *AppConfig) GetShowCoreLogs() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ShowCoreLogs
}

func (c *AppConfig) SetLogLimit(v int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.LogLimit = v
}

func (c *AppConfig) GetLogLimit() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.LogLimit
}

func (c *AppConfig) SetPluginsEnabled(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.PluginsEnabled = v
}

func (c *AppConfig) GetPluginsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.PluginsEnabled
}

func (c *AppConfig) SetPluginsDeveloper(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.PluginsDeveloper = v
}

func (c *AppConfig) GetPluginsDeveloper() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.PluginsDeveloper
}

func (c *AppConfig) SetCoreAutoRestart(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.CoreAutoRestart = v
}

func (c *AppConfig) GetCoreAutoRestart() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.CoreAutoRestart
}

func (c *AppConfig) SetFirstRunDone(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.FirstRunDone = v
}

func (c *AppConfig) GetFirstRunDone() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.FirstRunDone
}
