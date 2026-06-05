//go:build !noplugins

package plugins

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"sync"
)

// PluginState holds runtime state for a single plugin.
type PluginState struct {
	Enabled       bool   `json:"enabled"`
	SourceType    string `json:"source_type"`    // "folder" or "package"
	SourceURL     string `json:"source_url"`     // for packages
	LatestVersion string `json:"latest_version"` // cached from update check
}

// StateFile is the path to the plugin state JSON file.
func StateFile() string {
	return filepath.Join(PluginDir(), ".state.json")
}

// State manages plugin runtime state with file persistence.
type State struct {
	mu     sync.RWMutex
	states map[string]PluginState
}

// LoadState reads plugin state from disk.
func LoadState() (*State, error) {
	s := &State{states: make(map[string]PluginState)}
	data, err := os.ReadFile(StateFile())
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, &s.states); err != nil {
		return nil, err
	}
	return s, nil
}

// Save persists the state to disk.
func (s *State) Save() error {
	s.mu.RLock()
	data, err := json.MarshalIndent(s.states, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(StateFile(), data, 0600)
}

// Get returns the state for a plugin (zero value if not present).
func (s *State) Get(name string) PluginState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.states[name]
}

// Set updates the state for a plugin.
func (s *State) Set(name string, st PluginState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[name] = st
}

// Delete removes a plugin from state.
func (s *State) Delete(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.states, name)
}

// All returns a copy of all states.
func (s *State) All() map[string]PluginState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]PluginState, len(s.states))
	maps.Copy(out, s.states)
	return out
}
