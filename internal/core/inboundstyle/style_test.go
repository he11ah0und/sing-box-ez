package inboundstyle

import (
	"testing"
)

func TestDetect(t *testing.T) {
	cases := []struct {
		name     string
		inbounds []any
		want     Style
	}{
		{"empty", []any{}, StyleUndefined},
		{"nil", nil, StyleUndefined},
		{"client_mixed", []any{map[string]any{"type": "mixed"}}, StyleClient},
		{"client_tun", []any{map[string]any{"type": "tun"}}, StyleClient},
		{"client_mixed_tun", []any{map[string]any{"type": "tun"}, map[string]any{"type": "mixed"}}, StyleClient},
		{"server_tag", []any{map[string]any{"type": "mixed", "tag": "in"}}, StyleServer},
		{"server_other_type", []any{map[string]any{"type": "shadowsocks"}}, StyleServer},
		{"server_mixed_with_http", []any{map[string]any{"type": "mixed"}, map[string]any{"type": "http"}}, StyleServer},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Detect(tc.inbounds)
			if got != tc.want {
				t.Fatalf("Detect() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestApplyOverrideProxyOff(t *testing.T) {
	tree := map[string]any{
		"inbounds": []any{
			map[string]any{
				"type": "tun",
				"platform": map[string]any{
					"http_proxy": map[string]any{
						"enabled":     true,
						"server":      "127.0.0.1",
						"server_port": 2080,
					},
				},
			},
			map[string]any{"type": "mixed", "listen": "127.0.0.1", "listen_port": 2080},
		},
	}
	if err := ApplyOverride(tree, false, ""); err != nil {
		t.Fatal(err)
	}
	inbounds, ok := tree["inbounds"].([]any)
	if !ok || len(inbounds) != 1 {
		t.Fatalf("expected 1 inbound, got %v", tree["inbounds"])
	}
	tun := inbounds[0].(map[string]any)
	if tun["type"] != "tun" {
		t.Fatalf("expected tun inbound, got %v", tun["type"])
	}
	platform := tun["platform"].(map[string]any)
	httpProxy := platform["http_proxy"].(map[string]any)
	if httpProxy["enabled"] != false {
		t.Fatalf("expected http_proxy.enabled = false, got %v", httpProxy["enabled"])
	}
}

func TestApplyOverrideToClient(t *testing.T) {
	tree := map[string]any{
		"inbounds": []any{
			map[string]any{"type": "shadowsocks", "tag": "ss-in"},
		},
	}
	if err := ApplyOverride(tree, true, FallbackToClient); err != nil {
		t.Fatal(err)
	}
	inbounds := tree["inbounds"].([]any)
	if len(inbounds) != 2 {
		t.Fatalf("expected 2 inbounds, got %d", len(inbounds))
	}
	if inbounds[0].(map[string]any)["type"] != "tun" {
		t.Fatalf("expected first inbound tun, got %v", inbounds[0])
	}
	if inbounds[1].(map[string]any)["type"] != "mixed" {
		t.Fatalf("expected second inbound mixed, got %v", inbounds[1])
	}

	tree2 := map[string]any{}
	if err := ApplyOverride(tree2, false, FallbackToClient); err != nil {
		t.Fatal(err)
	}
	inbounds2 := tree2["inbounds"].([]any)
	if len(inbounds2) != 1 {
		t.Fatalf("expected 1 inbound when proxy off, got %d", len(inbounds2))
	}
}
