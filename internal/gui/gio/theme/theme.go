// Package theme provides YAML-driven light/dark themes for the GUI.
package theme

import (
	"fmt"
	"image/color"
	"math"
	"strconv"
	"strings"

	"gioui.org/widget/material"
)

// Variant selects the light or dark palette of a theme.
type Variant string

const (
	VariantLight Variant = "light"
	VariantDark  Variant = "dark"
)

// Mode selects how the active variant is chosen.
type Mode string

const (
	ModeSystem Mode = "system"
	ModeDark   Mode = "dark"
	ModeLight  Mode = "light"
)

// Palette holds the semantic colors used across the application.
type Palette struct {
	// Base Material palette.
	Bg             color.NRGBA
	Fg             color.NRGBA
	Surface        color.NRGBA
	SurfaceVariant color.NRGBA
	Primary        color.NRGBA
	PrimaryVariant color.NRGBA
	OnPrimary      color.NRGBA

	// Status / accent.
	Error      color.NRGBA
	Warning    color.NRGBA
	Success    color.NRGBA
	Info       color.NRGBA
	Disabled   color.NRGBA
	DisabledFg color.NRGBA

	// Generic overlays.
	Border   color.NRGBA
	Backdrop color.NRGBA
	Hover    color.NRGBA

	// Input.
	InputBg      color.NRGBA
	InputDirtyBg color.NRGBA
	InputBorder  color.NRGBA

	// Log view.
	LogDate   color.NRGBA
	LogSource color.NRGBA
	LogArrow  color.NRGBA
	LogDebug  color.NRGBA
	LogInfo   color.NRGBA
	LogWarn   color.NRGBA
	LogError  color.NRGBA

	LogBgDebug   color.NRGBA
	LogBgInfo    color.NRGBA
	LogBgWarn    color.NRGBA
	LogBgError   color.NRGBA
	LogBgDefault color.NRGBA

	// Profile cards.
	CardCached               color.NRGBA
	CardUncached             color.NRGBA
	CardCachedNoAutoUpdate   color.NRGBA
	CardUncachedNoAutoUpdate color.NRGBA

	// Core / privileges.
	CoreHighlight color.NRGBA
	StatusWarning color.NRGBA
	StatusOK      color.NRGBA
	Separator     color.NRGBA
}

// Theme wraps a material.Theme together with the semantic Palette.
type Theme struct {
	Material *material.Theme
	Dark     Palette
	Light    Palette
	def      *themeDef
}

// Current returns the active theme. It returns an empty theme if Init has not
// been called.
func Current() *Theme {
	if M == nil || M.current == nil {
		return &Theme{}
	}
	return M.current
}

// Colors returns the palette for the currently applied variant.
func (t *Theme) Colors() Palette {
	if t == nil {
		return Palette{}
	}
	// The variant is determined by the material palette background brightness.
	if isDark(t.Material.Palette.Bg) {
		return t.Dark
	}
	return t.Light
}

func (t *Theme) palette(v Variant) Palette {
	if v == VariantLight {
		return t.Light
	}
	return t.Dark
}

func isDark(c color.NRGBA) bool {
	// HSP perceived brightness.
	r := float64(c.R)
	g := float64(c.G)
	b := float64(c.B)
	return math.Sqrt(0.299*r*r+0.587*g*g+0.114*b*b) < 128
}

// rawPalette is the intermediate representation loaded from YAML.
type rawPalette struct {
	Bg             string `yaml:"bg"`
	Fg             string `yaml:"fg"`
	Surface        string `yaml:"surface"`
	SurfaceVariant string `yaml:"surface_variant"`
	Primary        string `yaml:"primary"`
	PrimaryVariant string `yaml:"primary_variant"`
	OnPrimary      string `yaml:"on_primary"`

	Error      string `yaml:"error"`
	Warning    string `yaml:"warning"`
	Success    string `yaml:"success"`
	Info       string `yaml:"info"`
	Disabled   string `yaml:"disabled"`
	DisabledFg string `yaml:"disabled_fg"`

	Border   string `yaml:"border"`
	Backdrop string `yaml:"backdrop"`
	Hover    string `yaml:"hover"`

	InputBg      string `yaml:"input_bg"`
	InputDirtyBg string `yaml:"input_dirty_bg"`
	InputBorder  string `yaml:"input_border"`

	LogDate   string `yaml:"log_date"`
	LogSource string `yaml:"log_source"`
	LogArrow  string `yaml:"log_arrow"`
	LogDebug  string `yaml:"log_debug"`
	LogInfo   string `yaml:"log_info"`
	LogWarn   string `yaml:"log_warn"`
	LogError  string `yaml:"log_error"`

	LogBgDebug   string `yaml:"log_bg_debug"`
	LogBgInfo    string `yaml:"log_bg_info"`
	LogBgWarn    string `yaml:"log_bg_warn"`
	LogBgError   string `yaml:"log_bg_error"`
	LogBgDefault string `yaml:"log_bg_default"`

	CardCached               string `yaml:"card_cached"`
	CardUncached             string `yaml:"card_uncached"`
	CardCachedNoAutoUpdate   string `yaml:"card_cached_no_auto_update"`
	CardUncachedNoAutoUpdate string `yaml:"card_uncached_no_auto_update"`

	CoreHighlight string `yaml:"core_highlight"`
	StatusWarning string `yaml:"status_warning"`
	StatusOK      string `yaml:"status_ok"`
	Separator     string `yaml:"separator"`
}

// parseHex parses a hex color string (#RGB, #RGBA, #RRGGBB, #RRGGBBAA).
func parseHex(s string) (color.NRGBA, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "#") {
		return color.NRGBA{}, fmt.Errorf("color %q must start with #", s)
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
		return color.NRGBA{}, fmt.Errorf("invalid hex color %q", s)
	}

	c := color.NRGBA{A: 255}
	var err error
	c.R, err = parseHexByte(s[0:2])
	if err != nil {
		return color.NRGBA{}, err
	}
	c.G, err = parseHexByte(s[2:4])
	if err != nil {
		return color.NRGBA{}, err
	}
	c.B, err = parseHexByte(s[4:6])
	if err != nil {
		return color.NRGBA{}, err
	}
	if len(s) == 8 {
		a, err := parseHexByte(s[6:8])
		if err != nil {
			return color.NRGBA{}, err
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

func rawToPalette(r rawPalette) (Palette, error) {
	var pal Palette
	fields := []struct {
		name string
		src  string
		dst  *color.NRGBA
	}{
		{"bg", r.Bg, &pal.Bg},
		{"fg", r.Fg, &pal.Fg},
		{"surface", r.Surface, &pal.Surface},
		{"surface_variant", r.SurfaceVariant, &pal.SurfaceVariant},
		{"primary", r.Primary, &pal.Primary},
		{"primary_variant", r.PrimaryVariant, &pal.PrimaryVariant},
		{"on_primary", r.OnPrimary, &pal.OnPrimary},
		{"error", r.Error, &pal.Error},
		{"warning", r.Warning, &pal.Warning},
		{"success", r.Success, &pal.Success},
		{"info", r.Info, &pal.Info},
		{"disabled", r.Disabled, &pal.Disabled},
		{"disabled_fg", r.DisabledFg, &pal.DisabledFg},
		{"border", r.Border, &pal.Border},
		{"backdrop", r.Backdrop, &pal.Backdrop},
		{"hover", r.Hover, &pal.Hover},
		{"input_bg", r.InputBg, &pal.InputBg},
		{"input_dirty_bg", r.InputDirtyBg, &pal.InputDirtyBg},
		{"input_border", r.InputBorder, &pal.InputBorder},
		{"log_date", r.LogDate, &pal.LogDate},
		{"log_source", r.LogSource, &pal.LogSource},
		{"log_arrow", r.LogArrow, &pal.LogArrow},
		{"log_debug", r.LogDebug, &pal.LogDebug},
		{"log_info", r.LogInfo, &pal.LogInfo},
		{"log_warn", r.LogWarn, &pal.LogWarn},
		{"log_error", r.LogError, &pal.LogError},
		{"log_bg_debug", r.LogBgDebug, &pal.LogBgDebug},
		{"log_bg_info", r.LogBgInfo, &pal.LogBgInfo},
		{"log_bg_warn", r.LogBgWarn, &pal.LogBgWarn},
		{"log_bg_error", r.LogBgError, &pal.LogBgError},
		{"log_bg_default", r.LogBgDefault, &pal.LogBgDefault},
		{"card_cached", r.CardCached, &pal.CardCached},
		{"card_uncached", r.CardUncached, &pal.CardUncached},
		{"card_cached_no_auto_update", r.CardCachedNoAutoUpdate, &pal.CardCachedNoAutoUpdate},
		{"card_uncached_no_auto_update", r.CardUncachedNoAutoUpdate, &pal.CardUncachedNoAutoUpdate},
		{"core_highlight", r.CoreHighlight, &pal.CoreHighlight},
		{"status_warning", r.StatusWarning, &pal.StatusWarning},
		{"status_ok", r.StatusOK, &pal.StatusOK},
		{"separator", r.Separator, &pal.Separator},
	}
	for _, f := range fields {
		if f.src == "" {
			continue
		}
		c, err := parseHex(f.src)
		if err != nil {
			return Palette{}, fmt.Errorf("%s: %w", f.name, err)
		}
		*f.dst = c
	}
	return pal, nil
}

func applyPalette(th *material.Theme, p Palette) {
	th.Palette.Bg = p.Bg
	th.Palette.Fg = p.Fg
	th.Palette.ContrastBg = p.Primary
	th.Palette.ContrastFg = p.OnPrimary
}
