// Package singboxconfig provides a parser and validator for sing-box configuration.
// It checks deprecated and removed fields against the official documentation.
package singboxconfig

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed schema.yaml
var schemaYAML []byte

var loadedSchema *Schema

func init() {
	var err error
	loadedSchema, err = LoadSchema(strings.NewReader(string(schemaYAML)))
	if err != nil {
		panic(fmt.Sprintf("singboxconfig: failed to load embedded schema: %v", err))
	}
}

// DeprecatedField describes a deprecated or removed configuration field.
type DeprecatedField struct {
	Path        string `json:"path" yaml:"path" msgpack:"path"`                      // JSON path, e.g. "experimental.clash_api.store_mode"
	Deprecated  string `json:"deprecated" yaml:"deprecated" msgpack:"deprecated"`    // version when deprecated
	Removed     string `json:"removed" yaml:"removed" msgpack:"removed"`             // version when removed (empty if not yet removed)
	Replacement string `json:"replacement" yaml:"replacement" msgpack:"replacement"` // what to use instead
}

// ValidationResult is the result of a configuration validation.
type ValidationResult struct {
	Warnings []DeprecatedField `json:"warnings" yaml:"warnings" msgpack:"warnings"`
	Errors   []DeprecatedField `json:"errors" yaml:"errors" msgpack:"errors"`
	Info     []string          `json:"info" yaml:"info" msgpack:"info"`
}

func (r *ValidationResult) String() string {
	var sb strings.Builder
	if len(r.Errors) > 0 {
		sb.WriteString(fmt.Sprintf("❌ ERRORS (removed fields, config will fail on newer sing-box): %d\n", len(r.Errors)))
		for _, e := range r.Errors {
			if e.Removed != "" {
				sb.WriteString(fmt.Sprintf("   [removed in %s] %s -> %s\n", e.Removed, e.Path, e.Replacement))
			} else {
				sb.WriteString(fmt.Sprintf("   [REMOVED] %s -> %s\n", e.Path, e.Replacement))
			}
		}
		sb.WriteString("\n")
	}
	if len(r.Warnings) > 0 {
		sb.WriteString(fmt.Sprintf("⚠️  WARNINGS (deprecated fields, still work but will be removed): %d\n", len(r.Warnings)))
		for _, w := range r.Warnings {
			removed := "not yet removed"
			if w.Removed != "" {
				removed = fmt.Sprintf("will be removed in %s", w.Removed)
			}
			sb.WriteString(fmt.Sprintf("   [deprecated in %s, %s] %s -> %s\n", w.Deprecated, removed, w.Path, w.Replacement))
		}
		sb.WriteString("\n")
	}
	if len(r.Info) > 0 {
		sb.WriteString(fmt.Sprintf("ℹ️  INFO: %d\n", len(r.Info)))
		for _, i := range r.Info {
			sb.WriteString(fmt.Sprintf("   %s\n", i))
		}
	}
	if len(r.Errors) == 0 && len(r.Warnings) == 0 && len(r.Info) == 0 {
		sb.WriteString("✅ No deprecated or removed fields detected.\n")
	}
	return sb.String()
}

// ConfigParser parses and validates sing-box configs.
type ConfigParser struct {
	result ValidationResult
	target Version
}

// NewConfigParser creates a parser that validates against the latest known sing-box version.
func NewConfigParser() *ConfigParser {
	latest, _ := ParseVersion(loadedSchema.SingboxLatest)
	return &ConfigParser{target: latest}
}

// NewConfigParserForVersion creates a parser that validates against a specific sing-box version.
func NewConfigParserForVersion(v string) (*ConfigParser, error) {
	latest, err := ParseVersion(v)
	if err != nil {
		return nil, fmt.Errorf("target version: %w", err)
	}
	return &ConfigParser{target: latest}, nil
}

// Parse разбирает JSON и проверяет deprecated поля
func (p *ConfigParser) Parse(data []byte) (*Config, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal root: %w", err)
	}

	cfg := &Config{raw: raw}
	p.walkObject("", loadedSchema.Fields, false, raw)

	// Semantic checks not expressible by field metadata.
	p.validateLegacyDNSServerAddress(raw)
	p.validateImplicitHTTPClient(raw)

	return cfg, nil
}

// Result возвращает результат валидации
func (p *ConfigParser) Result() ValidationResult {
	return p.result
}

func (p *ConfigParser) addWarn(d DeprecatedField) {
	p.result.Warnings = append(p.result.Warnings, d)
}

func (p *ConfigParser) addError(d DeprecatedField) {
	p.result.Errors = append(p.result.Errors, d)
}

func (p *ConfigParser) walkObject(path string, schemaFields map[string]*SchemaNode, additionalProperties bool, raw map[string]json.RawMessage) {
	for key, val := range raw {
		childPath := key
		if path != "" {
			childPath = path + "." + key
		}
		node, ok := schemaFields[key]
		if !ok {
			if additionalProperties {
				continue
			}
			p.addError(DeprecatedField{
				Path:        childPath,
				Replacement: "unknown field",
			})
			continue
		}
		p.checkNode(childPath, node)
		p.walkValue(childPath, node, val)
	}
}

func (p *ConfigParser) walkValue(path string, node *SchemaNode, raw json.RawMessage) {
	switch node.Type {
	case "object":
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			return
		}
		fields := p.resolveObjectFields(node, obj)
		p.walkObject(path, fields, node.AdditionalProperties, obj)
	case "array":
		var arr []json.RawMessage
		if err := json.Unmarshal(raw, &arr); err != nil {
			return
		}
		if node.Items == nil {
			return
		}
		for i, item := range arr {
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			p.walkValue(itemPath, node.Items, item)
		}
	}
}

func (p *ConfigParser) resolveObjectFields(node *SchemaNode, obj map[string]json.RawMessage) map[string]*SchemaNode {
	fields := make(map[string]*SchemaNode)
	for k, v := range node.Children {
		fields[k] = v
	}
	if node.OneOfBy == "" || len(node.OneOf) == 0 {
		return fields
	}

	discriminator := ""
	if discriminatorRaw, ok := obj[node.OneOfBy]; ok {
		_ = json.Unmarshal(discriminatorRaw, &discriminator)
	}

	// Always merge the default variant first so common fields are available,
	// then override with the matching typed variant.
	for _, variant := range node.OneOf {
		if len(variant.When) == 0 {
			for k, v := range variant.Fields {
				fields[k] = v
			}
			break
		}
	}
	for _, variant := range node.OneOf {
		if len(variant.When) == 0 {
			continue
		}
		match := true
		for k, want := range variant.When {
			if k != node.OneOfBy {
				// Only single-field discriminator supported for now.
				match = false
				break
			}
			if discriminator != want {
				match = false
				break
			}
		}
		if match {
			for k, v := range variant.Fields {
				fields[k] = v
			}
			break
		}
	}
	return fields
}

func (p *ConfigParser) checkNode(path string, node *SchemaNode) {
	if !node.RemovedV.Equal(Version{}) && node.RemovedV.LessOrEqual(p.target) {
		p.addError(DeprecatedField{
			Path:        path,
			Deprecated:  node.Deprecated,
			Removed:     node.Removed,
			Replacement: node.Replacement,
		})
		return
	}
	if !node.DeprecatedV.Equal(Version{}) && node.DeprecatedV.LessOrEqual(p.target) {
		p.addWarn(DeprecatedField{
			Path:        path,
			Deprecated:  node.Deprecated,
			Removed:     node.Removed,
			Replacement: node.Replacement,
		})
	}
}

func (p *ConfigParser) validateLegacyDNSServerAddress(raw map[string]json.RawMessage) {
	dnsRaw, ok := raw["dns"]
	if !ok {
		return
	}
	var dnsObj map[string]json.RawMessage
	if err := json.Unmarshal(dnsRaw, &dnsObj); err != nil {
		return
	}
	serversRaw, ok := dnsObj["servers"]
	if !ok {
		return
	}
	var servers []json.RawMessage
	if err := json.Unmarshal(serversRaw, &servers); err != nil {
		return
	}
	for i, srv := range servers {
		var srvObj map[string]json.RawMessage
		if err := json.Unmarshal(srv, &srvObj); err != nil {
			continue
		}
		addr, ok := srvObj["address"]
		if !ok {
			continue
		}
		var addrStr string
		if err := json.Unmarshal(addr, &addrStr); err != nil {
			continue
		}
		_ = addrStr
		p.addError(DeprecatedField{
			Path:        fmt.Sprintf("dns.servers[%d].address", i),
			Deprecated:  "1.12.0",
			Removed:     "1.14.0",
			Replacement: "type + server object (see migration guide)",
		})
	}
}

func (p *ConfigParser) validateImplicitHTTPClient(raw map[string]json.RawMessage) {
	routeRaw, ok := raw["route"]
	if !ok {
		return
	}
	var routeObj map[string]json.RawMessage
	if err := json.Unmarshal(routeRaw, &routeObj); err != nil {
		return
	}
	if !p.hasRemoteRuleSet(routeObj) {
		return
	}
	if _, hasHTTPClients := raw["http_clients"]; hasHTTPClients {
		return
	}
	if _, hasDefault := routeObj["default_http_client"]; hasDefault {
		return
	}
	p.addWarn(DeprecatedField{
		Path:        "route (implicit default HTTP client)",
		Deprecated:  "1.14.0",
		Removed:     "1.16.0",
		Replacement: "configure http_clients and route.default_http_client explicitly",
	})
}

func (p *ConfigParser) hasRemoteRuleSet(routeObj map[string]json.RawMessage) bool {
	rs, ok := routeObj["rule_set"]
	if !ok {
		return false
	}
	var ruleSets []map[string]json.RawMessage
	if err := json.Unmarshal(rs, &ruleSets); err != nil {
		return false
	}
	for _, rsItem := range ruleSets {
		t, ok := rsItem["type"]
		if !ok {
			continue
		}
		var typeStr string
		if err := json.Unmarshal(t, &typeStr); err == nil && typeStr == "remote" {
			return true
		}
	}
	return false
}

// Config represents a parsed config (wrapper around raw JSON).
type Config struct {
	raw map[string]json.RawMessage
}

// Raw returns the raw JSON map.
func (c *Config) Raw() map[string]json.RawMessage {
	return c.raw
}
