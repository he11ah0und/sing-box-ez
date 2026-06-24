package singboxconfig

import (
	"encoding/json"
	"testing"
)

func TestOverrideAcceptsValidEdit(t *testing.T) {
	input := []byte(`{"log":{"level":"info"}}`)
	out, ok, err := Override(input, func(tree map[string]any) bool {
		log, _ := tree["log"].(map[string]any)
		log["level"] = "debug"
		return true
	})
	if err != nil {
		t.Fatalf("Override: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	var cfg map[string]any
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	log := cfg["log"].(map[string]any)
	if log["level"] != "debug" {
		t.Fatalf("expected level=debug, got %v", log["level"])
	}
}

func TestOverrideRejectsUnknownField(t *testing.T) {
	input := []byte(`{"log":{"level":"info"}}`)
	_, ok, err := Override(input, func(tree map[string]any) bool {
		log := tree["log"].(map[string]any)
		log["fictional"] = true
		return true
	})
	if err != nil {
		t.Fatalf("Override: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for unknown field")
	}
}

func TestOverrideRejectsCallbackFalse(t *testing.T) {
	input := []byte(`{"log":{"level":"info"}}`)
	_, ok, err := Override(input, func(tree map[string]any) bool {
		return false
	})
	if err != nil {
		t.Fatalf("Override: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false when callback returns false")
	}
}
