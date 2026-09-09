package core

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestApplyCacheFileOverrideInjectsPerProfileCache(t *testing.T) {
	c := newUpdateTestController(t)

	out, err := c.applyCacheFileOverride([]byte(`{"log":{}}`), "sub")
	if err != nil {
		t.Fatalf("applyCacheFileOverride: %v", err)
	}
	var tree map[string]any
	if err := json.Unmarshal(out, &tree); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	cf, ok := tree["experimental"].(map[string]any)["cache_file"].(map[string]any)
	if !ok {
		t.Fatalf("cache_file section missing: %v", tree)
	}
	if cf["enabled"] != true {
		t.Fatalf("cache_file must be enabled: %v", cf)
	}
	wantPath := filepath.Join(c.cfg.DataDir, "cache", "sub.db")
	if cf["path"] != wantPath {
		t.Fatalf("cache path = %v, want %v", cf["path"], wantPath)
	}

	// A second pass must keep the injected section as-is.
	out2, err := c.applyCacheFileOverride(out, "sub")
	if err != nil {
		t.Fatalf("second pass: %v", err)
	}
	var tree2 map[string]any
	if err := json.Unmarshal(out2, &tree2); err != nil {
		t.Fatalf("unmarshal second pass: %v", err)
	}
	cf2 := tree2["experimental"].(map[string]any)["cache_file"].(map[string]any)
	if cf2["path"] != wantPath {
		t.Fatalf("second pass rewrote cache path: %v", cf2)
	}
}

func TestApplyCacheFileOverrideRespectsUserSection(t *testing.T) {
	c := newUpdateTestController(t)

	in := []byte(`{"experimental":{"cache_file":{"enabled":true,"path":"/tmp/user-cache.db","store_fakeip":true}}}`)
	out, err := c.applyCacheFileOverride(in, "sub")
	if err != nil {
		t.Fatalf("applyCacheFileOverride: %v", err)
	}
	var tree map[string]any
	if err := json.Unmarshal(out, &tree); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	cf := tree["experimental"].(map[string]any)["cache_file"].(map[string]any)
	if cf["path"] != "/tmp/user-cache.db" || cf["store_fakeip"] != true {
		t.Fatalf("user cache_file section must stay untouched: %v", cf)
	}
}

func TestApplyCacheFileOverrideNoName(t *testing.T) {
	c := newUpdateTestController(t)
	in := []byte(`{"log":{}}`)
	out, err := c.applyCacheFileOverride(in, "")
	if err != nil {
		t.Fatalf("applyCacheFileOverride: %v", err)
	}
	if string(out) != string(in) {
		t.Fatalf("empty config name must leave the config unchanged: %s", out)
	}
}
