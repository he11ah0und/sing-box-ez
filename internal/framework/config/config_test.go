package config

import (
	"strings"
	"testing"
)

func TestRegisterDisabled(t *testing.T) {
	s := NewSheet(SheetOptions{})

	enabled := s.Register([]string{"group", "enabled"}, TypeBool, true)
	disabled := s.Register([]string{"group", "disabled"}, TypeBool, false, WithDisabled(true))

	if enabled.IsDisabled() {
		t.Errorf("enabled cell reported disabled")
	}
	if !disabled.IsDisabled() {
		t.Errorf("disabled cell not reported disabled")
	}
}

func TestDisabledUpdateBlocked(t *testing.T) {
	s := NewSheet(SheetOptions{})
	cell := s.Register([]string{"feature"}, TypeBool, false, WithDisabled(true))

	if err := cell.Update(true); err == nil {
		t.Fatalf("expected error updating disabled cell")
	}
	if cell.Bool() != false {
		t.Errorf("disabled cell value changed, got %v want default false", cell.Bool())
	}
}

func TestDisabledIgnoredOnLoad(t *testing.T) {
	s := NewSheet(SheetOptions{})
	s.Register([]string{"feature"}, TypeBool, false, WithDisabled(true))
	s.Register([]string{"other"}, TypeBool, false)

	data := []byte(`
feature: true
other: true
`)
	if err := s.LoadYAML(data); err != nil {
		t.Fatalf("LoadYAML failed: %v", err)
	}

	if got := s.Bool("feature"); got != false {
		t.Errorf("disabled cell loaded from file, got %v want default false", got)
	}
	if got := s.Bool("other"); got != true {
		t.Errorf("enabled cell not loaded, got %v want true", got)
	}
}

func TestDisabledNotSaved(t *testing.T) {
	s := NewSheet(SheetOptions{})
	s.Register([]string{"feature"}, TypeBool, false, WithDisabled(true))
	s.Register([]string{"other"}, TypeBool, true)

	data, err := s.SaveYAML()
	if err != nil {
		t.Fatalf("SaveYAML failed: %v", err)
	}
	out := string(data)
	if strings.Contains(out, "feature") {
		t.Errorf("disabled key saved to YAML: %s", out)
	}
	if !strings.Contains(out, "other") {
		t.Errorf("enabled key missing from YAML: %s", out)
	}
}

func TestDisabledSubtreeIgnored(t *testing.T) {
	s := NewSheet(SheetOptions{})
	s.Register([]string{"parent", "child"}, TypeString, "default", WithDisabled(true))
	s.Register([]string{"parent", "sibling"}, TypeString, "default")

	data := []byte(`
parent:
  child: changed
  sibling: changed
`)
	if err := s.LoadYAML(data); err != nil {
		t.Fatalf("LoadYAML failed: %v", err)
	}
	if got := s.String("parent", "child"); got != "default" {
		t.Errorf("disabled subtree loaded, got %q want default", got)
	}
	if got := s.String("parent", "sibling"); got != "changed" {
		t.Errorf("enabled sibling not loaded, got %q", got)
	}

	saved, err := s.SaveYAML()
	if err != nil {
		t.Fatalf("SaveYAML failed: %v", err)
	}
	if strings.Contains(string(saved), "child") {
		t.Errorf("disabled subtree saved: %s", string(saved))
	}
}

func TestDisabledReturnsDefault(t *testing.T) {
	s := NewSheet(SheetOptions{})
	s.Register([]string{"feature"}, TypeInt, 42, WithDisabled(true))

	if got := s.Int("feature"); got != 42 {
		t.Errorf("disabled Int returned %d want 42", got)
	}
	cell, err := s.Get("feature")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got := cell.Any(); got != 42 {
		t.Errorf("disabled Any returned %v want 42", got)
	}
}

func TestUnknownKeysNotSaved(t *testing.T) {
	s := NewSheet(SheetOptions{})
	s.Register([]string{"known"}, TypeString, "value")

	data := []byte(`
known: changed
unknown: should-be-dropped
`)
	if err := s.LoadYAML(data); err != nil {
		t.Fatalf("LoadYAML failed: %v", err)
	}

	saved, err := s.SaveYAML()
	if err != nil {
		t.Fatalf("SaveYAML failed: %v", err)
	}
	out := string(saved)
	if !strings.Contains(out, "known") {
		t.Errorf("known key missing from YAML: %s", out)
	}
	if strings.Contains(out, "unknown") {
		t.Errorf("unknown key saved to YAML: %s", out)
	}
}

func TestDisabledCount(t *testing.T) {
	s := NewSheet(SheetOptions{})
	s.Register([]string{"a"}, TypeBool, false)
	s.Register([]string{"b"}, TypeBool, false, WithDisabled(true))
	s.Register([]string{"c", "c1"}, TypeBool, false)
	s.Register([]string{"c", "c2"}, TypeBool, false, WithDisabled(true))
	s.Register([]string{"d", "d1"}, TypeBool, false, WithDisabled(true))
	s.Register([]string{"d", "d2"}, TypeBool, false, WithDisabled(true))

	if got := s.DisabledCount(); got != 4 {
		t.Errorf("DisabledCount() = %d want 4", got)
	}
}
