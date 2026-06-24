package singboxconfig

import (
	"os"
	"testing"
)

func TestLoadEmbeddedSchema(t *testing.T) {
	f, err := os.Open("schema.yaml")
	if err != nil {
		t.Fatalf("open schema.yaml: %v", err)
	}
	defer f.Close()

	s, err := LoadSchema(f)
	if err != nil {
		t.Fatalf("LoadSchema: %v", err)
	}
	if s.Version == "" {
		t.Error("schema version is empty")
	}
	if s.SingboxLatest == "" {
		t.Error("singbox_latest is empty")
	}
	if len(s.Fields) == 0 {
		t.Error("schema has no fields")
	}
}

func TestBuildDictionary(t *testing.T) {
	f, err := os.Open("schema.yaml")
	if err != nil {
		t.Fatalf("open schema.yaml: %v", err)
	}
	defer f.Close()

	s, err := LoadSchema(f)
	if err != nil {
		t.Fatalf("LoadSchema: %v", err)
	}
	d, err := BuildDictionary(s)
	if err != nil {
		t.Fatalf("BuildDictionary: %v", err)
	}
	checks := []string{
		"log.level",
		"dns.servers[]",
		"dns.rules[].action",
		"route.rules[].sniff",
		"experimental.clash_api.store_mode",
		"inbounds[].type",
		"outbounds[].override_address",
	}
	for _, p := range checks {
		if d.Lookup(p) == nil {
			t.Errorf("expected path %q in dictionary", p)
		}
	}
}
