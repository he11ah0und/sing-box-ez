package plugins

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"sing-box-ez/internal/config"
)

// Manager manages loaded plugin engines and plugin metadata.
type Manager struct {
	mu      sync.RWMutex
	engines map[string]*Engine   // only loaded (enabled) plugins
	infos   map[string]*Manifest // all discovered plugins
	state   *State
	window  fyne.Window
	tabs    *container.AppTabs
	sink    func(string)
	cfg     *config.AppConfig
}

// NewManager creates a plugin manager.
func NewManager(w fyne.Window, tabs *container.AppTabs, cfg *config.AppConfig, logSink func(string)) *Manager {
	state, _ := LoadState()
	return &Manager{
		engines: make(map[string]*Engine),
		infos:   make(map[string]*Manifest),
		state:   state,
		window:  w,
		tabs:    tabs,
		sink:    logSink,
		cfg:     cfg,
	}
}

// Discover scans the plugins directory, loads metadata, restores state,
// and loads all enabled plugins into engines.
func (m *Manager) Discover() error {
	dir := PluginDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	type loadItem struct {
		mf        *Manifest
		entryPath string
	}
	var toLoad []loadItem

	for _, e := range entries {
		if !e.IsDir() || e.Name() == "" || e.Name()[0] == '.' {
			continue
		}
		pdir := filepath.Join(dir, e.Name())
		mf, err := LoadManifest(pdir)
		if err != nil {
			m.sink("[plugins] skip " + e.Name() + ": " + err.Error())
			continue
		}
		if mf.Entry == "" {
			mf.Entry = "main.lua"
		}

		// Restore runtime state.
		st := m.state.Get(mf.Name)
		mf.SourceType = st.SourceType
		if mf.SourceType == "" {
			mf.SourceType = "folder"
		}
		mf.SourceURL = st.SourceURL
		mf.Enabled = st.Enabled
		mf.LatestVersion = st.LatestVersion

		m.mu.Lock()
		m.infos[mf.Name] = mf
		m.mu.Unlock()

		if mf.Enabled {
			entryPath := filepath.Join(pdir, mf.Entry)
			if _, err := os.Stat(entryPath); err == nil {
				toLoad = append(toLoad, loadItem{mf: mf, entryPath: entryPath})
			} else {
				m.sink("[plugins] missing entrypoint for " + mf.Name)
			}
		}
	}

	for _, item := range toLoad {
		if err := m.Load(item.mf, item.entryPath); err != nil {
			m.sink("[plugins] failed to load " + item.mf.Name + ": " + err.Error())
		}
	}
	return nil
}

// loadLocked loads a single plugin into a new engine (must hold write lock).
func (m *Manager) loadLocked(mf *Manifest, entryPath string) error {
	if _, ok := m.engines[mf.Name]; ok {
		return fmt.Errorf("plugin %s already loaded", mf.Name)
	}
	builder := NewUIBuilder(m.window, m.tabs)
	engine, err := NewEngine(mf, builder, m.cfg, m.sink)
	if err != nil {
		return err
	}
	if err := engine.Load(entryPath); err != nil {
		engine.Close()
		return err
	}
	m.engines[mf.Name] = engine
	mf.Enabled = true
	m.sink("[plugins] loaded: " + mf.Name + " v" + mf.Version)
	return nil
}

// Load loads a single plugin into a new engine.
func (m *Manager) Load(mf *Manifest, entryPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.loadLocked(mf, entryPath)
}

// Unload unloads a plugin by name.
func (m *Manager) Unload(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if eng, ok := m.engines[name]; ok {
		eng.Close()
		delete(m.engines, name)
		if mf, ok := m.infos[name]; ok {
			mf.Enabled = false
		}
		m.sink("[plugins] unloaded: " + name)
	}
}

// Toggle enables or disables a plugin and persists the state.
func (m *Manager) Toggle(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mf, ok := m.infos[name]
	if !ok {
		return fmt.Errorf("plugin %s not found", name)
	}
	pdir := filepath.Join(PluginDir(), name)
	entryPath := filepath.Join(pdir, mf.Entry)

	if mf.Enabled {
		// Disable.
		if eng, ok := m.engines[name]; ok {
			eng.Close()
			delete(m.engines, name)
		}
		mf.Enabled = false
		m.state.Set(name, PluginState{
			Enabled:       false,
			SourceType:    mf.SourceType,
			SourceURL:     mf.SourceURL,
			LatestVersion: mf.LatestVersion,
		})
		_ = m.state.Save()
		m.sink("[plugins] disabled: " + name)
		return nil
	}

	// Enable.
	if _, err := os.Stat(entryPath); err != nil {
		return fmt.Errorf("missing entrypoint: %s", entryPath)
	}
	if err := m.loadLocked(mf, entryPath); err != nil {
		return err
	}
	m.state.Set(name, PluginState{
		Enabled:       true,
		SourceType:    mf.SourceType,
		SourceURL:     mf.SourceURL,
		LatestVersion: mf.LatestVersion,
	})
	_ = m.state.Save()
	return nil
}

// Reload unloads then loads a plugin again.
func (m *Manager) Reload(name string, mf *Manifest, entryPath string) error {
	m.Unload(name)
	return m.Load(mf, entryPath)
}

// CheckUpdate fetches the remote manifest and compares versions.
// Returns (hasUpdate, latestVersion, error).
func (m *Manager) CheckUpdate(name string) (bool, string, error) {
	m.mu.RLock()
	mf, ok := m.infos[name]
	m.mu.RUnlock()
	if !ok {
		return false, "", fmt.Errorf("plugin %s not found", name)
	}
	if mf.SourceType == "folder" {
		return false, "", fmt.Errorf("folder plugins cannot be auto-updated")
	}
	if mf.UpdateURL == "" {
		return false, "", fmt.Errorf("plugin %s has no update_url", name)
	}

	// Fetch the remote manifest.
	resp, err := http.Get(mf.UpdateURL)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Errorf("update check HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, "", err
	}
	var remote Manifest
	if err := json.Unmarshal(body, &remote); err != nil {
		return false, "", fmt.Errorf("invalid remote manifest: %w", err)
	}

	// Cache latest version.
	m.mu.Lock()
	mf.LatestVersion = remote.Version
	m.state.Set(name, PluginState{
		Enabled:       mf.Enabled,
		SourceType:    mf.SourceType,
		SourceURL:     mf.SourceURL,
		LatestVersion: remote.Version,
	})
	m.mu.Unlock()
	_ = m.state.Save()

	hasUpdate := remote.Version != "" && remote.Version != mf.Version
	return hasUpdate, remote.Version, nil
}

// CheckAllUpdates checks every package plugin that has an update_url.
func (m *Manager) CheckAllUpdates() {
	m.mu.RLock()
	var names []string
	for n, mf := range m.infos {
		if mf.SourceType == "package" && mf.UpdateURL != "" {
			names = append(names, n)
		}
	}
	m.mu.RUnlock()
	for _, name := range names {
		hasUpdate, latest, err := m.CheckUpdate(name)
		if err != nil {
			m.sink("[plugins] update check failed for " + name + ": " + err.Error())
			continue
		}
		if hasUpdate {
			m.sink("[plugins] update available for " + name + ": " + latest)
		} else {
			m.sink("[plugins] " + name + " is up to date")
		}
	}
}

// InstallFromURL downloads a plugin package and integrates it.
func (m *Manager) InstallFromURL(url string) error {
	mf, err := InstallFromURL(url)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.infos[mf.Name] = mf
	m.state.Set(mf.Name, PluginState{
		Enabled:       true,
		SourceType:    mf.SourceType,
		SourceURL:     mf.SourceURL,
		LatestVersion: "",
	})
	m.mu.Unlock()
	_ = m.state.Save()

	// Load it immediately.
	pdir := filepath.Join(PluginDir(), mf.Name)
	entryPath := filepath.Join(pdir, mf.Entry)
	if err := m.Load(mf, entryPath); err != nil {
		return fmt.Errorf("installed but failed to load: %w", err)
	}
	m.sink("[plugins] installed from package: " + mf.Name)
	return nil
}

// List returns the names of all discovered plugins.
func (m *Manager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.infos))
	for n := range m.infos {
		names = append(names, n)
	}
	return names
}

// GetManifest returns the full manifest (with runtime fields) for a plugin.
func (m *Manager) GetManifest(name string) *Manifest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if mf, ok := m.infos[name]; ok {
		return mf
	}
	return nil
}

// Close unloads all plugins and saves state.
func (m *Manager) Close() {
	m.mu.Lock()
	for _, eng := range m.engines {
		eng.Close()
	}
	m.engines = make(map[string]*Engine)
	m.mu.Unlock()
	_ = m.state.Save()
}
