package api

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"sing-box-ez/internal/singboxconfig"
)

// Backend selects which API transport the core should expose.
type Backend string

const (
	BackendClash   Backend = "clash"
	BackendSingBox Backend = "sing-box"
)

// Info holds the runtime parameters needed to connect to the core API.
type Info struct {
	Backend Backend
	Host    string
	Port    int
	Secret  string
}

// Addr returns the host:port address for the API listener.
func (i *Info) Addr() string {
	return net.JoinHostPort(i.Host, strconv.Itoa(i.Port))
}

// URL returns an HTTP URL for the API (used by the Clash API client).
func (i *Info) URL() string {
	return "http://" + i.Addr()
}

// VersionCutoff is the first sing-box version that ships the native sing-box
// API service. Earlier versions rely on the Clash API.
const VersionCutoff = "1.14.0"

// ApplyOverride rewrites the sing-box config to expose a local API on the
// provided host/port with the provided secret. It selects the Clash API for
// cores older than 1.14.0 and the sing-box API service for newer ones.
func ApplyOverride(data []byte, version, host string, port int, secret string) ([]byte, *Info, error) {
	parser, err := singboxconfig.NewConfigParserForVersion(version)
	if err != nil {
		parser = singboxconfig.NewConfigParser()
	}

	backend := BackendClash
	if v, err := singboxconfig.ParseVersion(version); err == nil {
		if cutoff, err := singboxconfig.ParseVersion(VersionCutoff); err == nil && !v.Less(cutoff) {
			backend = BackendSingBox
		}
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	info := &Info{Backend: backend, Host: host, Port: port, Secret: secret}

	out, ok, err := parser.Override(data, func(tree map[string]any) bool {
		switch backend {
		case BackendSingBox:
			applySingBoxAPI(tree, addr, secret)
		default:
			applyClashAPI(tree, addr, secret)
		}
		return true
	})
	if err != nil {
		return nil, nil, fmt.Errorf("apply API override: %w", err)
	}
	if !ok {
		// Validation reported unknown fields. This can happen when the schema
		// does not yet know about the selected API fields. Return the output
		// anyway but leave a hint for the caller.
	}
	return out, info, nil
}

func applyClashAPI(tree map[string]any, addr, secret string) {
	experimental := getOrCreateMap(tree, "experimental")
	experimental["clash_api"] = map[string]any{
		"external_controller": addr,
		"secret":              secret,
		"default_mode":        "rule",
	}
}

func applySingBoxAPI(tree map[string]any, addr, secret string) {
	services := getOrCreateSlice(tree, "services")
	services = append(services, map[string]any{
		"type":   "api",
		"listen": addr,
		"secret": secret,
	})
	tree["services"] = services
}

func getOrCreateMap(tree map[string]any, key string) map[string]any {
	if v, ok := tree[key].(map[string]any); ok {
		return v
	}
	if raw, ok := tree[key].(json.RawMessage); ok {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err == nil {
			tree[key] = m
			return m
		}
	}
	m := make(map[string]any)
	tree[key] = m
	return m
}

func getOrCreateSlice(tree map[string]any, key string) []any {
	if v, ok := tree[key].([]any); ok {
		return v
	}
	if raw, ok := tree[key].(json.RawMessage); ok {
		var s []any
		if err := json.Unmarshal(raw, &s); err == nil {
			tree[key] = s
			return s
		}
	}
	return nil
}
