//go:build !noplugins

package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Manifest describes a plugin and is read from manifest.json in the plugin directory.
type Manifest struct {
	// Name is the unique plugin identifier (directory name).
	Name string `json:"name"`
	// Version is the current plugin version in semver format.
	Version string `json:"version"`
	// Author is the plugin author name or contact.
	Author string `json:"author"`
	// Description is a short human-readable description.
	Description string `json:"description"`
	// Entry is the path to the Lua entry point, relative to the plugin directory.
	Entry string `json:"entrypoint"`
	// UpdateURL is an optional HTTPS URL that returns the latest manifest.json.
	UpdateURL string `json:"update_url,omitempty"`
	// Relations indicates what this plugin targets: "client", "server", or both.
	// In JSON this may be a string or an array of strings.
	Relations Relations `json:"relation,omitempty"`

	// --- Runtime fields (not persisted in manifest.json) ---
	SourceType    string `json:"-"` // "folder" or "package"
	SourceURL     string `json:"-"` // for packages: original download URL
	Enabled       bool   `json:"-"` // loaded or not
	LatestVersion string `json:"-"` // cached from update check
}

// Relations is a slice that unmarshals from either a single JSON string
// or a JSON array of strings.
type Relations []string

// UnmarshalJSON supports "client", ["client"], and ["client","server"].
func (r *Relations) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*r = []string{s}
		return nil
	}
	var arr []string
	if err := json.Unmarshal(data, &arr); err != nil {
		return fmt.Errorf("relation must be a string or an array of strings: %w", err)
	}
	*r = arr
	return nil
}

// MarshalJSON serialises back to a single string when there is one element,
// otherwise to an array.
func (r Relations) MarshalJSON() ([]byte, error) {
	if len(r) == 1 {
		return json.Marshal(r[0])
	}
	return json.Marshal([]string(r))
}

// Validate checks that the manifest contains the required fields.
func (m *Manifest) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("manifest name is required")
	}
	if m.Entry == "" {
		return fmt.Errorf("manifest entrypoint is required")
	}
	return nil
}

// LoadManifest reads and parses a manifest.json from the given plugin directory.
func LoadManifest(dir string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}
