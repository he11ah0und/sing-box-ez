//go:build noplugins

package plugins

import "sing-box-ez/internal/config"

type Manager struct{}

func NewManager(w any, tabs any, cfg *config.AppConfig, logSink func(string)) *Manager {
	return &Manager{}
}

func (m *Manager) Discover() error                                          { return nil }
func (m *Manager) Close() error                                             { return nil }
func (m *Manager) Load(mf *Manifest, entryPath string) error                { return nil }
func (m *Manager) Unload(name string) error                                 { return nil }
func (m *Manager) Toggle(name string) error                                 { return nil }
func (m *Manager) Reload(name string, mf *Manifest, entryPath string) error { return nil }
func (m *Manager) CheckUpdate(name string) error                            { return nil }
func (m *Manager) CheckAllUpdates()                                         {}
func (m *Manager) List() []string                                           { return nil }
func (m *Manager) GetManifest(name string) (*Manifest, error)               { return nil, nil }

func GeneratePluginTemplate(outDir, name, rel string) error { return nil }
func InstallFromURL(url string) (*Manifest, error)          { return nil, nil }

type UIBuilder struct{}
type Engine struct{}
type Manifest struct {
	ID         string
	Name       string
	Version    string
	Enabled    bool
	SourceType string
	SourceURL  string
}

func NewUIBuilder(w any, tabs any) *UIBuilder {
	return &UIBuilder{}
}

func NewEngine(m *Manifest, builder *UIBuilder, cfg *config.AppConfig, logSink func(string)) *Engine {
	return &Engine{}
}

func (e *Engine) Load(entryPath string) error { return nil }
func (e *Engine) Close() error                { return nil }

func GenerateDocs(outDir string) error     { return nil }
func GenerateLuaDefs(outDir string) error  { return nil }
func GenerateLuaTemplate() (string, error) { return "", nil }
