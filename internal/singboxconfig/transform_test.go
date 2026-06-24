package singboxconfig

import (
	"encoding/json"
	"testing"
)

func TestTransformRemovesField(t *testing.T) {
	input := []byte(`{"route":{"geoip":{}}}`)
	out, err := Transform(input, "1.7.0", "1.13.13")
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	route := cfg["route"].(map[string]any)
	if _, ok := route["geoip"]; ok {
		t.Fatal("expected geoip to be removed")
	}
}

func TestTransformRenamesField(t *testing.T) {
	input := []byte(`{"route":{"rules":[{"rule_set_ipcidr_match_source":true}]}}`)
	out, err := Transform(input, "1.9.0", "1.11.0")
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	rules := cfg["route"].(map[string]any)["rules"].([]any)
	rule := rules[0].(map[string]any)
	if _, ok := rule["rule_set_ipcidr_match_source"]; ok {
		t.Fatal("expected old key to be renamed")
	}
	if rule["rule_set_ip_cidr_match_source"] != true {
		t.Fatalf("expected rule_set_ip_cidr_match_source=true, got %v", rule["rule_set_ip_cidr_match_source"])
	}
}

func TestTransformKeepsFieldWhenTargetOlder(t *testing.T) {
	input := []byte(`{"route":{"geoip":{}}}`)
	out, err := Transform(input, "1.7.0", "1.7.0")
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	route := cfg["route"].(map[string]any)
	if _, ok := route["geoip"]; !ok {
		t.Fatal("expected geoip to be kept for same version")
	}
}
