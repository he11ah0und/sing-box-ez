//go:build !nogui

package wails

import (
	"fmt"
	"runtime"
	"slices"
	"strings"
)

// ConfigSpecEntry describes one user-visible config setting, derived from the
// "config" section of the project spec (internal/app/project.yaml).
type ConfigSpecEntry struct {
	// Path is the dot-joined config path (e.g. "core.log.level").
	Path string `json:"path"`
	// Type is "bool" | "int" | "string".
	Type string `json:"type"`
	// Control is the UI control hint ("text" | "number" | "switch" | "select" | "action").
	Control string `json:"control"`
	// Action is the backend action id for "action" controls (empty otherwise);
	// action entries carry no value and are run via RunConfigAction.
	Action string `json:"action"`
	// Confirm asks the UI to confirm before running an "action" control.
	Confirm bool `json:"confirm"`
	// Options holds the allowed values for "select" controls, nil otherwise.
	Options []any `json:"options"`
	// Min and Max bound int entries; nil when absent.
	Min *int `json:"min"`
	Max *int `json:"max"`
	// Platforms restricts the entry to the listed GOOS values; nil means all
	// platforms.
	Platforms []string `json:"platforms"`
}

// visibleConfigEntries returns the config entries exposed to the settings UI:
// disabled entries, entries without a UI control (those stay server-side
// only), and entries restricted to other platforms are excluded.
func (b *Bindings) visibleConfigEntries() []ConfigSpecEntry {
	spec := b.app.Spec
	if spec == nil {
		return nil
	}
	var out []ConfigSpecEntry
	for _, e := range spec.Config {
		if e.Disabled || e.Control == "" {
			continue
		}
		if len(e.Platforms) > 0 && !slices.Contains(e.Platforms, runtime.GOOS) {
			continue
		}
		out = append(out, ConfigSpecEntry{
			Path:      strings.Join(e.Path, "."),
			Type:      e.Type,
			Control:   e.Control,
			Action:    e.Action,
			Confirm:   e.Confirm,
			Options:   e.Options,
			Min:       e.Min,
			Max:       e.Max,
			Platforms: e.Platforms,
		})
	}
	return out
}

// GetConfigSpec returns the user-visible config schema so the frontend can
// generate the settings page from it.
func (b *Bindings) GetConfigSpec() []ConfigSpecEntry {
	return b.visibleConfigEntries()
}

// GetConfigValues returns the current values of exactly the visible entries,
// keyed by dot-joined path. Action entries carry no value (no Sheet cell) and
// are skipped.
func (b *Bindings) GetConfigValues() map[string]any {
	values := make(map[string]any)
	cfg := b.app.Controller.Config()
	for _, e := range b.visibleConfigEntries() {
		if e.Control == "action" {
			continue
		}
		cell := cfg.MustGet(strings.Split(e.Path, ".")...)
		switch e.Type {
		case "bool":
			values[e.Path] = cell.Bool()
		case "int":
			values[e.Path] = cell.Int()
		default:
			values[e.Path] = cell.String()
		}
	}
	return values
}

// SetConfigValues updates config cells by dot-joined path and persists the
// config. Only visible entries (see GetConfigSpec) may be set; unknown or
// server-side paths and type mismatches are rejected.
func (b *Bindings) SetConfigValues(values map[string]any) error {
	visible := make(map[string]ConfigSpecEntry)
	for _, e := range b.visibleConfigEntries() {
		if e.Control == "action" {
			// Action entries carry no value and are not settable.
			continue
		}
		visible[e.Path] = e
	}
	cfg := b.app.Controller.Config()
	for path, value := range values {
		entry, ok := visible[path]
		if !ok {
			return fmt.Errorf("config path %q is not settable", path)
		}
		// JSON numbers arrive as float64; int cells must store ints.
		if entry.Type == "int" {
			f, ok := value.(float64)
			if !ok {
				return fmt.Errorf("config path %q: expected number, got %T", path, value)
			}
			value = int(f)
		}
		if err := cfg.Set(strings.Split(path, "."), value); err != nil {
			return err
		}
	}
	if err := cfg.Save(); err != nil {
		b.toastErr(err)
		return err
	}
	b.emit("settings:changed", values)
	b.emitTheme()
	b.toastT("success", []string{"settings", "saved"})
	return nil
}
