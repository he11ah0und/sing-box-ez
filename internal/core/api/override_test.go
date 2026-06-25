package api

import (
	"encoding/json"
	"testing"

	"sing-box-ez/internal/singboxconfig"
)

func TestApplyOverrideClashAPI(t *testing.T) {
	cfg := []byte(`{"log":{"level":"info"}}`)
	out, info, err := ApplyOverride(cfg, "1.13.13", "127.0.0.1", 19090, "secret")
	if err != nil {
		t.Fatalf("ApplyOverride: %v", err)
	}
	if info.Backend != BackendClash {
		t.Fatalf("expected clash backend, got %s", info.Backend)
	}

	var tree map[string]any
	if err := json.Unmarshal(out, &tree); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	experimental := tree["experimental"].(map[string]any)
	clash := experimental["clash_api"].(map[string]any)
	if clash["external_controller"] != "127.0.0.1:19090" {
		t.Fatalf("unexpected external_controller: %v", clash["external_controller"])
	}
	if clash["secret"] != "secret" {
		t.Fatalf("unexpected secret: %v", clash["secret"])
	}

	// Ensure the output validates against the 1.13 schema.
	parser, _ := singboxconfig.NewConfigParserForVersion("1.13.13")
	if _, err := parser.Parse(out); err != nil {
		t.Fatalf("parse output: %v", err)
	}
}

func TestApplyOverrideSingBoxAPI(t *testing.T) {
	cfg := []byte(`{"log":{"level":"info"}}`)
	out, info, err := ApplyOverride(cfg, "1.14.0-alpha.34", "127.0.0.1", 19090, "secret")
	if err != nil {
		t.Fatalf("ApplyOverride: %v", err)
	}
	if info.Backend != BackendSingBox {
		t.Fatalf("expected sing-box backend, got %s", info.Backend)
	}

	var tree map[string]any
	if err := json.Unmarshal(out, &tree); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	services := tree["services"].([]any)
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	svc := services[0].(map[string]any)
	if svc["type"] != "api" {
		t.Fatalf("unexpected service type: %v", svc["type"])
	}
	if svc["listen"] != "127.0.0.1:19090" {
		t.Fatalf("unexpected listen: %v", svc["listen"])
	}

	parser, _ := singboxconfig.NewConfigParserForVersion("1.14.0")
	if _, err := parser.Parse(out); err != nil {
		t.Fatalf("parse output: %v", err)
	}
}
