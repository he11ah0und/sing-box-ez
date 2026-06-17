package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ConfigRecord represents a single subscription / config entry.
type ConfigRecord struct {
	Name                string    `json:"name"`
	URL                 string    `json:"url"`
	UpdateIntervalHours int       `json:"update_interval_hours"`
	LastUpdate          Timestamp `json:"last_update"`
	// Parent identifies who created this config: "user" for user-created,
	// or "pl-{plugin_id}" for plugin-created configs.
	Parent string `json:"parent"`
}

// Timestamp is a wrapper around time.Time for custom JSON unmarshalling.
type Timestamp struct {
	time.Time
}

func (t Timestamp) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte("null"), nil
	}
	return t.Time.MarshalJSON()
}

func (t *Timestamp) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == `""` {
		t.Time = time.Time{}
		return nil
	}
	return t.Time.UnmarshalJSON(data)
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
	return time.Since(r.LastUpdate.Time) > time.Duration(r.UpdateIntervalHours)*time.Hour
}

type AppConfig struct {
	// Legacy single-config fields kept for backwards compatibility
	SingBoxURL          string `json:"singbox_url,omitempty"`
	UpdateIntervalHours int    `json:"update_interval_hours"`

	RunAsAdmin           bool   `json:"run_as_admin"`
	ShowLogs             bool   `json:"show_logs"`
	WatchCoreLogs        bool   `json:"watch_core_logs"`
	LogLimit             int    `json:"log_limit"`
	LogLevel             string `json:"log_level"`
	Language             string `json:"language"`
	PluginsEnabled       bool   `json:"plugins_enabled"`
	PluginsDeveloper     bool   `json:"plugins_developer"`
	CoreAutoRestart                 bool      `json:"core_auto_restart"`
	DesktopNotifications            bool      `json:"desktop_notifications"`
	AutoCheckSelfUpdates            bool      `json:"auto_check_self_updates"`
	AutoCheckCoreUpdates            bool      `json:"auto_check_core_updates"`
	StartupUpdateCheckIntervalHours int       `json:"startup_update_check_interval_hours"`
	LastSelfUpdateCheck             Timestamp `json:"last_self_update_check"`
	LastCoreUpdateCheck             Timestamp `json:"last_core_update_check"`

	DataDir string `json:"-"`

	mu       sync.RWMutex
	profiles *Profiles
}

func defaultAppConfig() *AppConfig {
	return &AppConfig{
		UpdateIntervalHours:             2,
		RunAsAdmin:                      false,
		ShowLogs:                        false,
		WatchCoreLogs:                   true,
		LogLimit:                        100,
		LogLevel:                        "info",
		CoreAutoRestart:                 true,
		DesktopNotifications:            true,
		AutoCheckSelfUpdates:            true,
		AutoCheckCoreUpdates:            true,
		StartupUpdateCheckIntervalHours: 2,
	}
}

// rawConfig is used to read legacy files that may still contain configs / active_name.
type rawConfig struct {
	AppConfig
	Configs    []ConfigRecord `json:"configs"`
	ActiveName string         `json:"active_name"`
}

func Load(dataDir string) (*AppConfig, error) {
	cfg, err := loadSettings(dataDir)
	if err != nil {
		return nil, err
	}
	cfg.DataDir = dataDir

	profiles, err := LoadProfiles(dataDir)
	if err != nil {
		return nil, err
	}
	cfg.profiles = profiles

	// Migrate legacy configs stored in config.json if profiles.json is empty.
	if len(profiles.Configs) == 0 {
		legacy, err := loadRawSettings(dataDir)
		if err == nil && (len(legacy.Configs) > 0 || legacy.ActiveName != "") {
			profiles.Configs = legacy.Configs
			profiles.ActiveName = legacy.ActiveName
			if profiles.Configs == nil {
				profiles.Configs = []ConfigRecord{}
			}
			_ = profiles.Save(dataDir)
		}
		// Also migrate legacy single URL into the new list if needed.
		if cfg.SingBoxURL != "" && len(profiles.Configs) == 0 {
			profiles.Configs = append(profiles.Configs, ConfigRecord{
				Name:                "default",
				URL:                 cfg.SingBoxURL,
				UpdateIntervalHours: cfg.UpdateIntervalHours,
				Parent:              "user",
			})
			profiles.ActiveName = "default"
			cfg.SingBoxURL = ""
			_ = profiles.Save(dataDir)
			_ = cfg.Save()
		}
	}

	return cfg, nil
}

func loadSettings(dataDir string) (*AppConfig, error) {
	data, err := os.ReadFile(filepath.Join(dataDir, "config.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return defaultAppConfig(), nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return defaultAppConfig(), nil
	}
	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func loadRawSettings(dataDir string) (*rawConfig, error) {
	data, err := os.ReadFile(filepath.Join(dataDir, "config.json"))
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return &rawConfig{}, nil
	}
	var cfg rawConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
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
	if c.DataDir == "" {
		return os.ErrInvalid
	}
	if err := os.WriteFile(filepath.Join(c.DataDir, "config.json"), data, 0600); err != nil {
		return err
	}
	if c.profiles != nil {
		return c.profiles.Save(c.DataDir)
	}
	return nil
}

func (c *AppConfig) GetConfigs() []ConfigRecord {
	if c.profiles == nil {
		return nil
	}
	return c.profiles.GetConfigs()
}

func (c *AppConfig) GetActiveConfig() *ConfigRecord {
	if c.profiles == nil {
		return nil
	}
	return c.profiles.GetActiveConfig()
}

func (c *AppConfig) GetActiveName() string {
	if c.profiles == nil {
		return ""
	}
	c.profiles.mu.RLock()
	defer c.profiles.mu.RUnlock()
	return c.profiles.ActiveName
}

func (c *AppConfig) GetConfigByName(name string) *ConfigRecord {
	if c.profiles == nil {
		return nil
	}
	return c.profiles.GetConfigByName(name)
}

// GetConfigsByParent returns all configs created by the given parent.
func (c *AppConfig) GetConfigsByParent(parent string) []ConfigRecord {
	if c.profiles == nil {
		return nil
	}
	return c.profiles.GetConfigsByParent(parent)
}

// GetConfigByNameAndParent returns a config only if it matches both name and parent.
func (c *AppConfig) GetConfigByNameAndParent(name, parent string) *ConfigRecord {
	if c.profiles == nil {
		return nil
	}
	return c.profiles.GetConfigByNameAndParent(name, parent)
}

func (c *AppConfig) SetActiveName(name string) {
	if c.profiles == nil {
		return
	}
	c.profiles.SetActiveName(name)
}

func (c *AppConfig) AddConfig(rec ConfigRecord) {
	if c.profiles == nil {
		return
	}
	c.profiles.AddConfig(rec)
}

func (c *AppConfig) UpdateConfig(name string, rec ConfigRecord) {
	if c.profiles == nil {
		return
	}
	c.profiles.UpdateConfig(name, rec)
}

func (c *AppConfig) RemoveConfig(name string) {
	if c.profiles == nil {
		return
	}
	c.profiles.RemoveConfig(name)
}

func (c *AppConfig) SetLastUpdateFor(name string, t time.Time) {
	if c.profiles == nil {
		return
	}
	c.profiles.SetLastUpdateFor(name, t)
}

func (c *AppConfig) SetDefaultUpdateInterval(hours int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.UpdateIntervalHours = hours
}

func (c *AppConfig) RenameConfig(oldName, newName string) {
	if c.profiles == nil {
		return
	}
	c.profiles.RenameConfig(oldName, newName)
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

func (c *AppConfig) SetWatchCoreLogs(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.WatchCoreLogs = v
}

func (c *AppConfig) GetWatchCoreLogs() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.WatchCoreLogs
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

func (c *AppConfig) SetLogLevel(v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.LogLevel = v
}

func (c *AppConfig) GetLogLevel() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.LogLevel == "" {
		return "info"
	}
	return c.LogLevel
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

func (c *AppConfig) SetDesktopNotifications(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.DesktopNotifications = v
}

func (c *AppConfig) GetDesktopNotifications() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.DesktopNotifications
}

func (c *AppConfig) SetLanguage(v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Language = v
}

func (c *AppConfig) GetLanguage() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Language
}


func (c *AppConfig) SetAutoCheckSelfUpdates(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.AutoCheckSelfUpdates = v
}

func (c *AppConfig) GetAutoCheckSelfUpdates() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.AutoCheckSelfUpdates
}

func (c *AppConfig) SetAutoCheckCoreUpdates(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.AutoCheckCoreUpdates = v
}

func (c *AppConfig) GetAutoCheckCoreUpdates() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.AutoCheckCoreUpdates
}

func (c *AppConfig) SetStartupUpdateCheckIntervalHours(v int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.StartupUpdateCheckIntervalHours = v
}

func (c *AppConfig) GetStartupUpdateCheckIntervalHours() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.StartupUpdateCheckIntervalHours <= 0 {
		return 2
	}
	return c.StartupUpdateCheckIntervalHours
}

func (c *AppConfig) SetLastSelfUpdateCheck(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.LastSelfUpdateCheck = Timestamp{Time: t}
}

func (c *AppConfig) GetLastSelfUpdateCheck() Timestamp {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.LastSelfUpdateCheck
}

func (c *AppConfig) SetLastCoreUpdateCheck(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.LastCoreUpdateCheck = Timestamp{Time: t}
}

func (c *AppConfig) GetLastCoreUpdateCheck() Timestamp {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.LastCoreUpdateCheck
}
