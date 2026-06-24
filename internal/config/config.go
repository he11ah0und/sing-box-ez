// Package config holds the sing-box-ez application configuration.
//
// Settings are stored as a typed cell sheet (see framework/config) in
// config.yaml. Profiles (subscription records) are stored in profiles.yaml.
package config

import (
	"errors"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	fwconfig "sing-box-ez/internal/framework/config"
	"sing-box-ez/internal/framework/fs"
)

const (
	// ConfigTypeRemote is a URL-based config that is downloaded periodically.
	ConfigTypeRemote = "remote"
	// ConfigTypeLocal is a config file edited and stored locally.
	ConfigTypeLocal = "local"
)

// ConfigRecord represents a single subscription / config entry.
type ConfigRecord struct {
	Name                string    `json:"name" yaml:"name"`
	URL                 string    `json:"url" yaml:"url"`
	Type                string    `json:"type" yaml:"type"`
	UpdateIntervalHours int       `json:"update_interval_hours" yaml:"update_interval_hours"`
	LastUpdate          Timestamp `json:"last_update" yaml:"last_update"`
	// Parent identifies who created this config: "user" for user-created,
	// or "pl-{plugin_id}" for plugin-created configs.
	Parent string `json:"parent" yaml:"parent"`
	// AutoUpdate controls whether the config is updated automatically.
	// nil means "enabled" for backward compatibility.
	AutoUpdate *bool `json:"auto_update" yaml:"auto_update"`
	// Hash is the SHA-256 hex digest of the last known config content.
	Hash string `json:"hash" yaml:"hash"`
}

// IsAutoUpdate reports whether automatic updates are enabled for this record.
// Missing/nil value defaults to true.
func (r *ConfigRecord) IsAutoUpdate() bool {
	if r.AutoUpdate == nil {
		return true
	}
	return *r.AutoUpdate
}

// IsLocal reports whether this record is a locally edited config.
// Records with an empty Type and empty URL are treated as local for backward
// compatibility.
func (r *ConfigRecord) IsLocal() bool {
	if r.Type == ConfigTypeLocal {
		return true
	}
	return r.Type == "" && r.URL == ""
}

// Timestamp is a wrapper around time.Time for custom JSON/YAML unmarshalling.
type Timestamp struct {
	time.Time
}

// MarshalJSON implements json.Marshaler.
func (t Timestamp) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte("null"), nil
	}
	return t.Time.MarshalJSON()
}

// UnmarshalJSON implements json.Unmarshaler.
func (t *Timestamp) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == `""` {
		t.Time = time.Time{}
		return nil
	}
	return t.Time.UnmarshalJSON(data)
}

// MarshalYAML implements yaml.Marshaler.
func (t Timestamp) MarshalYAML() (any, error) {
	if t.IsZero() {
		return nil, nil
	}
	return t.Time.Format(time.RFC3339), nil
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (t *Timestamp) UnmarshalYAML(n *yaml.Node) error {
	if n.Tag == "!!null" || n.Value == "" || n.Value == "null" {
		t.Time = time.Time{}
		return nil
	}
	var s string
	if err := n.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return err
	}
	t.Time = parsed
	return nil
}

// NextUpdate returns the next scheduled update time for this record.
func (r *ConfigRecord) NextUpdate() time.Time {
	if !r.IsAutoUpdate() || r.LastUpdate.IsZero() {
		return time.Time{}
	}
	return r.LastUpdate.Add(time.Duration(r.UpdateIntervalHours) * time.Hour)
}

// ShouldUpdate reports whether this config is due for an update.
func (r *ConfigRecord) ShouldUpdate() bool {
	if !r.IsAutoUpdate() {
		return false
	}
	if r.LastUpdate.IsZero() {
		return true
	}
	return time.Since(r.LastUpdate.Time) > time.Duration(r.UpdateIntervalHours)*time.Hour
}

// AppConfig wraps the framework config sheet and adds application-specific
// profiles and the data directory path.
type AppConfig struct {
	*fwconfig.Sheet
	Root     fs.Directory
	DataDir  string
	Profiles *Profiles
}

// Load reads settings and profiles from root and returns the loaded config.
func Load(root fs.Directory, dataDir string, sheet *fwconfig.Sheet) (fwconfig.Config, error) {
	if err := LoadYAML(root, sheet); err != nil {
		return nil, err
	}

	profiles, err := LoadProfiles(root)
	if err != nil {
		return nil, err
	}

	return &AppConfig{
		Sheet:    sheet,
		Root:     root,
		DataDir:  dataDir,
		Profiles: profiles,
	}, nil
}

// LoadYAML reads config.yaml into the sheet. Missing file is not an error.
func LoadYAML(root fs.Directory, sheet *fwconfig.Sheet) error {
	data, err := root.File("config.yaml").Read()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return sheet.LoadYAML(data)
}

// SaveYAML writes the sheet to config.yaml.
func SaveYAML(root fs.Directory, sheet *fwconfig.Sheet) error {
	data, err := sheet.SaveYAML()
	if err != nil {
		return err
	}
	return root.File("config.yaml").AtomicWrite(data, 0600)
}

// Save persists both settings and profiles.
func (c *AppConfig) Save() error {
	if c.Root == nil {
		return os.ErrInvalid
	}
	if err := SaveYAML(c.Root, c.Sheet); err != nil {
		return err
	}
	if c.Profiles != nil {
		return c.Profiles.Save(c.Root)
	}
	return nil
}

// GetActiveConfig returns the currently selected profile.
func (c *AppConfig) GetActiveConfig() *ConfigRecord {
	if c.Profiles == nil {
		return nil
	}
	return c.Profiles.GetActiveConfig()
}

// GetConfigs returns all profiles.
func (c *AppConfig) GetConfigs() []ConfigRecord {
	if c.Profiles == nil {
		return nil
	}
	return c.Profiles.GetConfigs()
}

// GetActiveName returns the active profile name.
func (c *AppConfig) GetActiveName() string {
	if c.Profiles == nil {
		return ""
	}
	return c.Profiles.ActiveName
}

// GetConfigByName returns a profile by name.
func (c *AppConfig) GetConfigByName(name string) *ConfigRecord {
	if c.Profiles == nil {
		return nil
	}
	return c.Profiles.GetConfigByName(name)
}

// GetConfigsByParent returns profiles created by the given parent.
func (c *AppConfig) GetConfigsByParent(parent string) []ConfigRecord {
	if c.Profiles == nil {
		return nil
	}
	return c.Profiles.GetConfigsByParent(parent)
}

// GetConfigByNameAndParent returns a profile matching both name and parent.
func (c *AppConfig) GetConfigByNameAndParent(name, parent string) *ConfigRecord {
	if c.Profiles == nil {
		return nil
	}
	return c.Profiles.GetConfigByNameAndParent(name, parent)
}

// SetActiveName sets the active profile.
func (c *AppConfig) SetActiveName(name string) {
	if c.Profiles == nil {
		return
	}
	c.Profiles.SetActiveName(name)
}

// AddConfig adds a profile.
func (c *AppConfig) AddConfig(rec ConfigRecord) {
	if c.Profiles == nil {
		return
	}
	c.Profiles.AddConfig(rec)
}

// UpdateConfig updates a profile.
func (c *AppConfig) UpdateConfig(name string, rec ConfigRecord) {
	if c.Profiles == nil {
		return
	}
	c.Profiles.UpdateConfig(name, rec)
}

// RemoveConfig removes a profile.
func (c *AppConfig) RemoveConfig(name string) {
	if c.Profiles == nil {
		return
	}
	c.Profiles.RemoveConfig(name)
}

// SetLastUpdateFor updates the last update timestamp of a profile.
func (c *AppConfig) SetLastUpdateFor(name string, t time.Time) {
	if c.Profiles == nil {
		return
	}
	c.Profiles.SetLastUpdateFor(name, t)
}

// RenameConfig renames a profile.
func (c *AppConfig) RenameConfig(oldName, newName string) {
	if c.Profiles == nil {
		return
	}
	c.Profiles.RenameConfig(oldName, newName)
}
