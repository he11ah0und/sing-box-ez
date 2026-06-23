package theme

import (
	"embed"
	"fmt"
	"sort"
	"strings"

	"gioui.org/widget/material"
	"gopkg.in/yaml.v3"

	"sing-box-ez/internal/framework/fs"
)

// themeDef is the raw YAML representation of a theme file.
type themeDef struct {
	Name        string     `yaml:"name"`
	DisplayName string     `yaml:"display_name"`
	Dark        rawPalette `yaml:"dark"`
	Light       rawPalette `yaml:"light"`
}

// Manager loads embedded and user themes and applies them to a material.Theme.
type Manager struct {
	material      *material.Theme
	defaultThemes map[string]*Theme
	userThemes    map[string]*Theme
	current       *Theme
	currentName   string
	currentMode   Mode
	dataDir       string
}

// M is the global theme manager. It is set by Init.
var M *Manager

// Init creates the global theme manager, loads themes and applies the default.
func Init(th *material.Theme, dataDir string, embedFS embed.FS) error {
	m := &Manager{
		material:      th,
		defaultThemes: make(map[string]*Theme),
		userThemes:    make(map[string]*Theme),
		dataDir:       dataDir,
	}
	if err := m.Load(fs.Embed(embedFS)); err != nil {
		return err
	}
	M = m
	return nil
}

// NewManager creates a theme manager bound to the given material theme.
func NewManager(th *material.Theme, dataDir string) *Manager {
	return &Manager{
		material:      th,
		defaultThemes: make(map[string]*Theme),
		userThemes:    make(map[string]*Theme),
		dataDir:       dataDir,
	}
}

// Load reads themes from the embedded FS and from dataDir/themes/*.yaml.
func (m *Manager) Load(defaultFS fs.FS) error {
	if err := m.loadFromDir(defaultFS.Root().Subdir("themes"), m.defaultThemes, true); err != nil {
		return fmt.Errorf("load embedded themes: %w", err)
	}
	userDir := fs.NewOS(m.dataDir).Root().Subdir("themes")
	if userDir.Exists() {
		if err := m.loadFromDir(userDir, m.userThemes, false); err != nil {
			return fmt.Errorf("load user themes: %w", err)
		}
	}
	return nil
}

func (m *Manager) loadFromDir(dir fs.Directory, into map[string]*Theme, isDefault bool) error {
	entries, err := dir.ReadDir()
	if err != nil {
		return err
	}
	for _, e := range entries {
		if _, isDir := e.(fs.Directory); isDir || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		file, ok := e.(fs.File)
		if !ok {
			continue
		}
		data, err := file.Read()
		if err != nil {
			return err
		}
		var def themeDef
		if err := yaml.Unmarshal(data, &def); err != nil {
			return fmt.Errorf("parse %s: %w", e.Name(), err)
		}
		if def.Name == "" {
			def.Name = strings.TrimSuffix(e.Name(), ".yaml")
		}
		if isDefault {
			if _, ok := into[def.Name]; ok {
				return fmt.Errorf("duplicate default theme %q", def.Name)
			}
		} else {
			if _, reserved := m.defaultThemes[def.Name]; reserved {
				// User themes cannot override defaults.
				continue
			}
			if _, ok := into[def.Name]; ok {
				return fmt.Errorf("duplicate user theme %q", def.Name)
			}
		}
		darkPal, err := rawToPalette(def.Dark)
		if err != nil {
			return fmt.Errorf("theme %q dark %w", def.Name, err)
		}
		lightPal, err := rawToPalette(def.Light)
		if err != nil {
			return fmt.Errorf("theme %q light %w", def.Name, err)
		}
		into[def.Name] = &Theme{
			Material: m.material,
			Dark:     darkPal,
			Light:    lightPal,
			def:      &def,
		}
	}
	return nil
}

// Names returns the sorted list of available theme names (defaults first, then user).
func (m *Manager) Names() []string {
	names := make([]string, 0, len(m.defaultThemes)+len(m.userThemes))
	for n := range m.defaultThemes {
		names = append(names, n)
	}
	for n := range m.userThemes {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Current returns the active theme.
func (m *Manager) Current() *Theme { return m.current }

// CurrentName returns the active theme name.
func (m *Manager) CurrentName() string { return m.currentName }

// CurrentMode returns the active mode.
func (m *Manager) CurrentMode() Mode { return m.currentMode }

// Apply sets the active theme and variant. An empty name falls back to "default".
func (m *Manager) Apply(name string, mode Mode) error {
	if name == "" {
		name = "default"
	}
	t, ok := m.themeByName(name)
	if !ok {
		return fmt.Errorf("theme %q not found", name)
	}
	variant := m.resolveVariant(mode)
	pal := t.palette(variant)
	applyPalette(m.material, pal)
	m.current = t
	m.currentName = name
	m.currentMode = mode
	return nil
}

func (m *Manager) themeByName(name string) (*Theme, bool) {
	if t, ok := m.defaultThemes[name]; ok {
		return t, true
	}
	if t, ok := m.userThemes[name]; ok {
		return t, true
	}
	return nil, false
}

func (m *Manager) resolveVariant(mode Mode) Variant {
	switch mode {
	case ModeDark:
		return VariantDark
	case ModeLight:
		return VariantLight
	default:
		return SystemVariant()
	}
}

// MustApply is like Apply but panics on error. Useful for initial setup.
func (m *Manager) MustApply(name string, mode Mode) {
	if err := m.Apply(name, mode); err != nil {
		panic(err)
	}
}
