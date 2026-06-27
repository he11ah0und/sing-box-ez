// Package inboundstyle classifies sing-box inbounds into client/server/undefined
// styles and applies client-side transformations.
package inboundstyle

import (
	"encoding/json"
	"fmt"
)

// Style describes the detected role of a config based on its inbounds.
type Style string

const (
	// StyleClient means all inbounds are mixed/tun inbounds without tags.
	StyleClient Style = "client"
	// StyleServer means at least one inbound is not a plain mixed/tun inbound.
	StyleServer Style = "server"
	// StyleUndefined means there are no inbounds to inspect.
	StyleUndefined Style = "undefined"
)

// Fallback values stored on ConfigRecord.
const (
	FallbackIgnore   = "ignore"
	FallbackToClient = "to_client"
)

// Detect returns the style of the provided inbounds slice.
func Detect(inbounds []any) Style {
	if len(inbounds) == 0 {
		return StyleUndefined
	}
	for _, raw := range inbounds {
		in, ok := raw.(map[string]any)
		if !ok {
			return StyleServer
		}
		t, _ := in["type"].(string)
		if t != "mixed" && t != "tun" {
			return StyleServer
		}
		if tag, _ := in["tag"].(string); tag != "" {
			return StyleServer
		}
	}
	return StyleClient
}

// DetectFromConfig returns the style by reading the "inbounds" key from the config tree.
func DetectFromConfig(tree map[string]any) Style {
	raw, ok := tree["inbounds"]
	if !ok {
		return StyleUndefined
	}
	inbounds, ok := raw.([]any)
	if !ok {
		return StyleUndefined
	}
	return Detect(inbounds)
}

// ApplyOverride mutates the config tree according to the proxy toggle and fallback type.
func ApplyOverride(tree map[string]any, proxyEnabled bool, fallbackType string) error {
	switch fallbackType {
	case FallbackToClient:
		tree["inbounds"] = buildClientInbounds(proxyEnabled)
		return nil
	case FallbackIgnore, "":
		// Keep existing inbounds, only apply proxy toggle rules.
	default:
		return fmt.Errorf("unknown fallback_type %q", fallbackType)
	}

	if !proxyEnabled {
		removeMixedInbounds(tree)
		disableTUNHTTPProxy(tree)
	}
	return nil
}

func removeMixedInbounds(tree map[string]any) {
	raw, ok := tree["inbounds"]
	if !ok {
		return
	}
	inbounds, ok := raw.([]any)
	if !ok {
		return
	}
	filtered := make([]any, 0, len(inbounds))
	for _, item := range inbounds {
		in, ok := item.(map[string]any)
		if !ok {
			filtered = append(filtered, item)
			continue
		}
		if t, _ := in["type"].(string); t == "mixed" {
			continue
		}
		filtered = append(filtered, item)
	}
	if len(filtered) == 0 {
		delete(tree, "inbounds")
	} else {
		tree["inbounds"] = filtered
	}
}

func disableTUNHTTPProxy(tree map[string]any) {
	raw, ok := tree["inbounds"]
	if !ok {
		return
	}
	inbounds, ok := raw.([]any)
	if !ok {
		return
	}
	for _, item := range inbounds {
		in, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if t, _ := in["type"].(string); t != "tun" {
			continue
		}
		platform, ok := in["platform"].(map[string]any)
		if !ok {
			platform = make(map[string]any)
			in["platform"] = platform
		}
		httpProxy, ok := platform["http_proxy"].(map[string]any)
		if !ok {
			httpProxy = make(map[string]any)
			platform["http_proxy"] = httpProxy
		}
		httpProxy["enabled"] = false
	}
}

func buildClientInbounds(proxyEnabled bool) []any {
	tun := map[string]any{
		"type": "tun",
		"address": []string{
			"172.19.0.1/30",
			"fdfe:dcba:9876::1/126",
		},
		"auto_route":               true,
		"endpoint_independent_nat": false,
		"mtu":                      9000,
		"platform": map[string]any{
			"http_proxy": map[string]any{
				"enabled":     proxyEnabled,
				"server":      "127.0.0.1",
				"server_port": 2080,
			},
		},
		"stack":        "system",
		"strict_route": false,
	}
	if !proxyEnabled {
		return []any{tun}
	}
	mixed := map[string]any{
		"type":        "mixed",
		"listen":      "127.0.0.1",
		"listen_port": 2080,
		"users":       []any{},
	}
	return []any{tun, mixed}
}

// ParseTree parses raw JSON bytes into a config tree.
func ParseTree(data []byte) (map[string]any, error) {
	var tree map[string]any
	if err := json.Unmarshal(data, &tree); err != nil {
		return nil, err
	}
	return tree, nil
}

// MarshalTree serializes a config tree back to JSON bytes.
func MarshalTree(tree map[string]any) ([]byte, error) {
	return json.Marshal(tree)
}
