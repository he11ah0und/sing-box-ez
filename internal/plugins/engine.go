//go:build !noplugins

package plugins

import (
	"fmt"
	"path/filepath"

	lua "github.com/yuin/gopher-lua"
	"sing-box-ez/internal/config"
)

// Engine wraps a Lua VM for a single plugin.
type Engine struct {
	L        *lua.LState
	manifest *Manifest
	builder  *UIBuilder
	sink     func(string)
	cfg      *config.AppConfig
}

// NewEngine creates a new Lua engine for the given plugin.
func NewEngine(m *Manifest, builder *UIBuilder, cfg *config.AppConfig, logSink func(string)) (*Engine, error) {
	L := lua.NewState()
	e := &Engine{
		L:        L,
		manifest: m,
		builder:  builder,
		sink:     logSink,
		cfg:      cfg,
	}
	registerHTTP(L)
	registerLog(L, logSink)
	registerConfig(L, cfg, m.Name)
	e.initUI()
	return e, nil
}

// Load runs the plugin entry point Lua script.
func (e *Engine) Load(entryPath string) error {
	if err := e.L.DoFile(entryPath); err != nil {
		return fmt.Errorf("plugin %s load failed: %w", e.manifest.Name, err)
	}
	return nil
}

// Close destroys the Lua state.
func (e *Engine) Close() {
	e.L.Close()
}

// PluginDir returns the plugins data directory.
func PluginDir() string {
	return filepath.Join("sing-box-ez-data", "plugins")
}
