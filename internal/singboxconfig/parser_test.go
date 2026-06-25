package singboxconfig

import (
	"encoding/json"
	"testing"
)

func TestParserDetectsUnknownField(t *testing.T) {
	cfg := map[string]any{
		"log": map[string]any{
			"level":     "info",
			"fictional": true,
		},
	}
	data, _ := json.Marshal(cfg)
	p := NewConfigParser()
	if _, err := p.Parse(data); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	res := p.Result()
	if len(res.Errors) == 0 {
		t.Fatalf("expected unknown field error")
	}
	found := false
	for _, e := range res.Errors {
		if e.Path == "log.fictional" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected error path log.fictional, got %+v", res.Errors)
	}
}

func TestParserDetectsRemovedField(t *testing.T) {
	cfg := map[string]any{
		"route": map[string]any{
			"geoip": map[string]any{},
		},
	}
	data, _ := json.Marshal(cfg)
	p := NewConfigParser()
	if _, err := p.Parse(data); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	res := p.Result()
	if len(res.Errors) == 0 {
		t.Fatalf("expected removed field error")
	}
	if res.Errors[0].Path != "route.geoip" {
		t.Fatalf("unexpected error: %+v", res.Errors[0])
	}
}

func TestParserDetectsDeprecatedField(t *testing.T) {
	cfg := map[string]any{
		"dns": map[string]any{
			"rules": []any{
				map[string]any{"rule_set_ip_cidr_accept_empty": true},
			},
		},
	}
	data, _ := json.Marshal(cfg)
	p := NewConfigParser()
	if _, err := p.Parse(data); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	res := p.Result()
	if len(res.Warnings) == 0 {
		t.Fatalf("expected deprecated field warning")
	}
	if res.Warnings[0].Path != "dns.rules[0].rule_set_ip_cidr_accept_empty" {
		t.Fatalf("unexpected warning: %+v", res.Warnings[0])
	}
}

func TestParserTargetVersionSkipsFutureDeprecation(t *testing.T) {
	cfg := map[string]any{
		"route": map[string]any{
			"geoip": map[string]any{},
		},
	}
	data, _ := json.Marshal(cfg)
	p, err := NewConfigParserForVersion("1.7.0")
	if err != nil {
		t.Fatalf("NewConfigParserForVersion: %v", err)
	}
	if _, err := p.Parse(data); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	res := p.Result()
	if len(res.Errors) > 0 || len(res.Warnings) > 0 {
		t.Fatalf("expected no issues for version 1.7.0, got %+v", res)
	}
}

func TestParserLegacyDNSAddress(t *testing.T) {
	cfg := map[string]any{
		"dns": map[string]any{
			"servers": []any{
				map[string]any{"address": "8.8.8.8"},
			},
		},
	}
	data, _ := json.Marshal(cfg)
	p := NewConfigParser()
	if _, err := p.Parse(data); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	res := p.Result()
	found := false
	for _, e := range res.Errors {
		if e.Path == "dns.servers[0].address" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected legacy dns address error, got %+v", res.Errors)
	}
}

func TestParserImplicitHTTPClient(t *testing.T) {
	cfg := map[string]any{
		"route": map[string]any{
			"rule_set": []any{
				map[string]any{"type": "remote", "url": "https://example.com/ruleset.json"},
			},
		},
	}
	data, _ := json.Marshal(cfg)
	p := NewConfigParser()
	if _, err := p.Parse(data); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	res := p.Result()
	found := false
	for _, w := range res.Warnings {
		if w.Path == "route (implicit default HTTP client)" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected implicit HTTP client warning, got %+v", res.Warnings)
	}
}
