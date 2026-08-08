// Package themes loads embedded and user YAML color themes for the GUI.
package themes

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed *.yaml
var embeddedFS embed.FS

// Theme holds the dark and light palettes for a single theme.
type Theme struct {
	Name        string
	DisplayName string
	Dark        map[string]string
	Light       map[string]string
}

// Collection is a loaded set of themes.
type Collection struct {
	themes  map[string]*Theme
	dataDir string
}

type themeDef struct {
	Name        string            `yaml:"name"`
	DisplayName string            `yaml:"display_name"`
	Dark        map[string]string `yaml:"dark"`
	Light       map[string]string `yaml:"light"`
}

// Load reads embedded themes and, optionally, user themes from dataDir/themes.
func Load(dataDir string) (*Collection, error) {
	c := &Collection{
		themes:  make(map[string]*Theme),
		dataDir: dataDir,
	}
	if err := c.loadEmbedded(); err != nil {
		return nil, err
	}
	if err := c.loadUser(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Collection) loadEmbedded() error {
	entries, err := embeddedFS.ReadDir(".")
	if err != nil {
		return fmt.Errorf("read embedded themes: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		data, err := embeddedFS.ReadFile(e.Name())
		if err != nil {
			return fmt.Errorf("read embedded theme %q: %w", e.Name(), err)
		}
		if err := c.parseFile(e.Name(), data, true); err != nil {
			return err
		}
	}
	return nil
}

func (c *Collection) loadUser() error {
	userDir := filepath.Join(c.dataDir, "themes")
	entries, err := os.ReadDir(userDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read user themes dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(userDir, e.Name())
		// #nosec G304 -- e.Name() is a base name from os.ReadDir inside the
		// fixed app themes dir; it cannot escape userDir.
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read user theme %q: %w", e.Name(), err)
		}
		if err := c.parseFile(e.Name(), data, false); err != nil {
			return err
		}
	}
	return nil
}

func (c *Collection) parseFile(fileName string, data []byte, isDefault bool) error {
	var def themeDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return fmt.Errorf("parse theme %q: %w", fileName, err)
	}
	if def.Name == "" {
		def.Name = strings.TrimSuffix(fileName, ".yaml")
	}
	if _, exists := c.themes[def.Name]; exists {
		if isDefault {
			return fmt.Errorf("duplicate default theme %q", def.Name)
		}
		// User theme overriding a default is ignored.
		return nil
	}
	c.themes[def.Name] = &Theme{
		Name:        def.Name,
		DisplayName: def.DisplayName,
		Dark:        normalizeColors(def.Dark),
		Light:       normalizeColors(def.Light),
	}
	return nil
}

// Names returns the sorted list of available theme names.
func (c *Collection) Names() []string {
	names := make([]string, 0, len(c.themes))
	for n := range c.themes {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Get returns a theme by name.
func (c *Collection) Get(name string) (*Theme, bool) {
	t, ok := c.themes[name]
	return t, ok
}

// Resolve returns the effective CSS color map for the given theme, mode and OS dark state.
func (c *Collection) Resolve(name, mode string, isDark bool) map[string]string {
	if name == "" {
		name = "default"
	}
	t, ok := c.themes[name]
	if !ok {
		t, ok = c.themes["default"]
		if !ok {
			return nil
		}
	}
	switch mode {
	case "light":
		return t.Light
	case "dark":
		return t.Dark
	default:
		if isDark {
			return t.Dark
		}
		return t.Light
	}
}

// RGBA is a parsed sRGBA color value.
type RGBA struct {
	R, G, B, A uint8
}

// ParseHex parses a hex color string (#RGB, #RGBA, #RRGGBB, #RRGGBBAA).
func ParseHex(s string) (RGBA, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "#") {
		return RGBA{}, fmt.Errorf("color %q must start with #", s)
	}
	s = s[1:]
	switch len(s) {
	case 3, 4:
		expanded := make([]byte, 0, len(s)*2)
		for _, b := range []byte(s) {
			expanded = append(expanded, b, b)
		}
		s = string(expanded)
	case 6, 8:
		// ok
	default:
		return RGBA{}, fmt.Errorf("invalid hex color %q", s)
	}

	c := RGBA{A: 255}
	var err error
	c.R, err = parseHexByte(s[0:2])
	if err != nil {
		return RGBA{}, err
	}
	c.G, err = parseHexByte(s[2:4])
	if err != nil {
		return RGBA{}, err
	}
	c.B, err = parseHexByte(s[4:6])
	if err != nil {
		return RGBA{}, err
	}
	if len(s) == 8 {
		a, err := parseHexByte(s[6:8])
		if err != nil {
			return RGBA{}, err
		}
		c.A = a
	}
	return c, nil
}

func parseHexByte(s string) (uint8, error) {
	v, err := strconv.ParseUint(s, 16, 8)
	if err != nil {
		return 0, fmt.Errorf("invalid hex byte %q: %w", s, err)
	}
	return uint8(v), nil
}

// normalizeKey converts YAML underscore keys to CSS-variable-friendly hyphen keys
// and applies semantic aliases so the default theme works with the Svelte frontend.
func normalizeKey(key string) string {
	if alias, ok := keyAliases[key]; ok {
		return alias
	}
	return strings.ReplaceAll(key, "_", "-")
}

func normalizeColors(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[normalizeKey(k)] = v
	}
	return out
}

var keyAliases = map[string]string{
	"fg":                           "text",
	"surface_variant":              "surface-variant",
	"primary_variant":              "primary-hover",
	"on_primary":                   "on-primary",
	"error":                        "danger",
	"disabled_fg":                  "text-muted",
	"input_bg":                     "input-bg",
	"input_dirty_bg":               "input-dirty-bg",
	"input_border":                 "input-border",
	"log_date":                     "log-date",
	"log_source":                   "log-source",
	"log_arrow":                    "log-arrow",
	"log_debug":                    "log-debug",
	"log_info":                     "log-info",
	"log_warn":                     "log-warn",
	"log_error":                    "log-error",
	"log_bg_debug":                 "log-bg-debug",
	"log_bg_info":                  "log-bg-info",
	"log_bg_warn":                  "log-bg-warn",
	"log_bg_error":                 "log-bg-error",
	"log_bg_default":               "log-bg-default",
	"card_cached":                  "card-cached",
	"card_uncached":                "card-uncached",
	"card_cached_no_auto_update":   "card-cached-no-auto-update",
	"card_uncached_no_auto_update": "card-uncached-no-auto-update",
	"core_highlight":               "core-highlight",
	"status_warning":               "status-warning",
	"status_ok":                    "status-ok",
}
