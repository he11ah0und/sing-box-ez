// Package singboxconfig provides a parser and validator for sing-box configuration.
// It checks deprecated and removed fields against the official documentation.
package singboxconfig

import (
	"encoding/json"
	"fmt"
	"strings"
)

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

// ConfigParser парсер и валидатор конфига sing-box
type ConfigParser struct {
	result ValidationResult
}

// NewConfigParser создаёт новый парсер
func NewConfigParser() *ConfigParser {
	return &ConfigParser{}
}

// Parse разбирает JSON и проверяет deprecated поля
func (p *ConfigParser) Parse(data []byte) (*Config, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal root: %w", err)
	}

	cfg := &Config{raw: raw}

	// Валидация корневых секций
	p.validateRoot(raw)

	// Валидация experimental
	if v, ok := raw["experimental"]; ok {
		p.validateExperimental(v)
	}

	// Валидация DNS
	if v, ok := raw["dns"]; ok {
		p.validateDNS(v)
	}

	// Валидация route
	if v, ok := raw["route"]; ok {
		p.validateRoute(v)
	}

	// Валидация inbounds
	if v, ok := raw["inbounds"]; ok {
		p.validateInbounds(v)
	}

	// Валидация outbounds
	if v, ok := raw["outbounds"]; ok {
		p.validateOutbounds(v)
	}

	// Валидация cache_file
	if v, ok := raw["cache_file"]; ok {
		p.validateCacheFile(v)
	}

	return cfg, nil
}

// Result возвращает результат валидации
func (p *ConfigParser) Result() ValidationResult {
	return p.result
}

// --- внутренние валидаторы ---

func (p *ConfigParser) validateRoot(raw map[string]json.RawMessage) {
	p.validateLegacyDNSServerAddress(raw)
	p.validateImplicitHTTPClient(raw)
}

func (p *ConfigParser) validateLegacyDNSServerAddress(raw map[string]json.RawMessage) {
	// Проверяем legacy DNS server format по наличию строкового address в dns.servers
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
		// Это legacy формат: address как строка
		p.addError(DeprecatedField{
			Path:        fmt.Sprintf("dns.servers[%d].address", i),
			Deprecated:  "1.12.0",
			Removed:     "1.14.0",
			Replacement: "type + server object (see migration guide)",
		})
	}
}

func (p *ConfigParser) validateImplicitHTTPClient(raw map[string]json.RawMessage) {
	// Проверяем implicit HTTP client (deprecated 1.14.0)
	// Если есть remote rule-sets но нет http_clients
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

func (p *ConfigParser) validateExperimental(raw json.RawMessage) {
	var exp map[string]json.RawMessage
	if err := json.Unmarshal(raw, &exp); err != nil {
		return
	}
	if clashRaw, ok := exp["clash_api"]; ok {
		var clash map[string]json.RawMessage
		if err := json.Unmarshal(clashRaw, &clash); err != nil {
			return
		}
		checkDeprecated := func(field, deprecated, removed, replacement string) {
			if _, ok := clash[field]; ok {
				p.addWarn(DeprecatedField{
					Path:        "experimental.clash_api." + field,
					Deprecated:  deprecated,
					Removed:     removed,
					Replacement: replacement,
				})
			}
		}
		checkDeprecated("store_mode", "1.8.0", "", "cache_file.enabled")
		checkDeprecated("store_selected", "1.8.0", "", "cache_file.enabled")
		checkDeprecated("store_fakeip", "1.8.0", "", "cache_file.store_fakeip")
		checkDeprecated("cache_file", "1.8.0", "", "cache_file.enabled / cache_file.path")
		checkDeprecated("cache_id", "1.8.0", "", "cache_file.cache_id")
	}
}

func (p *ConfigParser) validateDNS(raw json.RawMessage) {
	var dns map[string]json.RawMessage
	if err := json.Unmarshal(raw, &dns); err != nil {
		return
	}
	if _, ok := dns["independent_cache"]; ok {
		p.addWarn(DeprecatedField{
			Path:        "dns.independent_cache",
			Deprecated:  "1.14.0",
			Removed:     "1.16.0",
			Replacement: "(always enabled by default, no longer needed)",
		})
	}
	rulesRaw, ok := dns["rules"]
	if !ok {
		return
	}
	var rules []map[string]json.RawMessage
	if err := json.Unmarshal(rulesRaw, &rules); err != nil {
		return
	}
	for i, rule := range rules {
		p.validateDNSRule(rule, i)
	}
}

func (p *ConfigParser) validateDNSRule(rule map[string]json.RawMessage, i int) {
	// outbound (deprecated 1.12.0, removed 1.14.0)
	if _, ok := rule["outbound"]; ok {
		p.addError(DeprecatedField{
			Path:        fmt.Sprintf("dns.rules[%d].outbound", i),
			Deprecated:  "1.12.0",
			Removed:     "1.14.0",
			Replacement: "domain_resolver in dial fields or route.default_domain_resolver",
		})
	}
	// strategy (deprecated 1.14.0, removed 1.16.0)
	if _, ok := rule["strategy"]; ok {
		p.addWarn(DeprecatedField{
			Path:        fmt.Sprintf("dns.rules[%d].strategy", i),
			Deprecated:  "1.14.0",
			Removed:     "1.16.0",
			Replacement: "use server strategy or route action",
		})
	}
	// rule_set_ip_cidr_accept_empty (deprecated 1.14.0, removed 1.16.0)
	if _, ok := rule["rule_set_ip_cidr_accept_empty"]; ok {
		p.addWarn(DeprecatedField{
			Path:        fmt.Sprintf("dns.rules[%d].rule_set_ip_cidr_accept_empty", i),
			Deprecated:  "1.14.0",
			Removed:     "1.16.0",
			Replacement: "use match_response",
		})
	}
	// geoip / geosite (deprecated 1.8.0, removed 1.12.0)
	if _, ok := rule["geoip"]; ok {
		p.addError(DeprecatedField{
			Path:        fmt.Sprintf("dns.rules[%d].geoip", i),
			Deprecated:  "1.8.0",
			Removed:     "1.12.0",
			Replacement: "rule-set",
		})
	}
	if _, ok := rule["geosite"]; ok {
		p.addError(DeprecatedField{
			Path:        fmt.Sprintf("dns.rules[%d].geosite", i),
			Deprecated:  "1.8.0",
			Removed:     "1.12.0",
			Replacement: "rule-set",
		})
	}
	// Legacy Address Filter Fields (ip_cidr, ip_is_private без match_response)
	p.validateDNSAddressFilter(rule, i)
}

func (p *ConfigParser) validateDNSAddressFilter(rule map[string]json.RawMessage, i int) {
	hasMatchResponse := false
	if mr, ok := rule["match_response"]; ok {
		var mrBool bool
		if err := json.Unmarshal(mr, &mrBool); err == nil && mrBool {
			hasMatchResponse = true
		}
	}
	if hasMatchResponse {
		return
	}
	if _, ok := rule["ip_cidr"]; ok {
		p.addWarn(DeprecatedField{
			Path:        fmt.Sprintf("dns.rules[%d].ip_cidr", i),
			Deprecated:  "1.14.0",
			Removed:     "1.16.0",
			Replacement: "match_response + ip_cidr",
		})
	}
	if _, ok := rule["ip_is_private"]; ok {
		p.addWarn(DeprecatedField{
			Path:        fmt.Sprintf("dns.rules[%d].ip_is_private", i),
			Deprecated:  "1.14.0",
			Removed:     "1.16.0",
			Replacement: "match_response + ip_is_private",
		})
	}
}

func (p *ConfigParser) validateRoute(raw json.RawMessage) {
	var route map[string]json.RawMessage
	if err := json.Unmarshal(raw, &route); err != nil {
		return
	}
	if rulesRaw, ok := route["rules"]; ok {
		var rules []map[string]json.RawMessage
		if err := json.Unmarshal(rulesRaw, &rules); err != nil {
			return
		}
		for i, rule := range rules {
			if _, ok := rule["geoip"]; ok {
				p.addError(DeprecatedField{
					Path:        fmt.Sprintf("route.rules[%d].geoip", i),
					Deprecated:  "1.8.0",
					Removed:     "1.12.0",
					Replacement: "rule-set",
				})
			}
			if _, ok := rule["geosite"]; ok {
				p.addError(DeprecatedField{
					Path:        fmt.Sprintf("route.rules[%d].geosite", i),
					Deprecated:  "1.8.0",
					Removed:     "1.12.0",
					Replacement: "rule-set",
				})
			}
			if _, ok := rule["rule_set_ipcidr_match_source"]; ok {
				p.addError(DeprecatedField{
					Path:        fmt.Sprintf("route.rules[%d].rule_set_ipcidr_match_source", i),
					Deprecated:  "1.10.0",
					Removed:     "1.11.0",
					Replacement: "rule_set_ip_cidr_match_source",
				})
			}
			// inbound legacy fields на уровне route rules (sniff, domain_strategy)
			if _, ok := rule["sniff"]; ok {
				p.addError(DeprecatedField{
					Path:        fmt.Sprintf("route.rules[%d].sniff", i),
					Deprecated:  "1.11.0",
					Removed:     "1.13.0",
					Replacement: "rule action sniff",
				})
			}
		}
	}
	if rsRaw, ok := route["rule_set"]; ok {
		var ruleSets []map[string]json.RawMessage
		if err := json.Unmarshal(rsRaw, &ruleSets); err != nil {
			return
		}
		for i, rs := range ruleSets {
			if _, ok := rs["download_detour"]; ok {
				p.addWarn(DeprecatedField{
					Path:        fmt.Sprintf("route.rule_set[%d].download_detour", i),
					Deprecated:  "1.14.0",
					Removed:     "1.16.0",
					Replacement: "http_client",
				})
			}
		}
	}
}

func (p *ConfigParser) validateInbounds(raw json.RawMessage) {
	var inbounds []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &inbounds); err != nil {
		return
	}
	for i, in := range inbounds {
		var inType string
		if t, ok := in["type"]; ok {
			json.Unmarshal(t, &inType)
		}
		if inType == "tun" {
			check := func(field, deprecated, removed, replacement string) {
				if _, ok := in[field]; ok {
					p.addError(DeprecatedField{
						Path:        fmt.Sprintf("inbounds[%d].%s", i, field),
						Deprecated:  deprecated,
						Removed:     removed,
						Replacement: replacement,
					})
				}
			}
			check("inet4_address", "1.10.0", "1.12.0", "address")
			check("inet6_address", "1.10.0", "1.12.0", "address")
			check("inet4_route_address", "1.10.0", "1.11.0", "route_address")
			check("inet6_route_address", "1.10.0", "1.11.0", "route_address")
			check("inet4_route_exclude_address", "1.10.0", "1.11.0", "route_exclude_address")
			check("inet6_route_exclude_address", "1.10.0", "1.11.0", "route_exclude_address")
			check("gso", "1.11.0", "1.13.0", "(removed, no replacement)")
		}
		// Legacy inbound fields (sniff, domain_strategy на уровне inbound)
		if _, ok := in["sniff"]; ok {
			p.addError(DeprecatedField{
				Path:        fmt.Sprintf("inbounds[%d].sniff", i),
				Deprecated:  "1.11.0",
				Removed:     "1.13.0",
				Replacement: "rule action sniff",
			})
		}
		if _, ok := in["domain_strategy"]; ok {
			p.addError(DeprecatedField{
				Path:        fmt.Sprintf("inbounds[%d].domain_strategy", i),
				Deprecated:  "1.11.0",
				Removed:     "1.13.0",
				Replacement: "rule action domain_resolver",
			})
		}
	}
}

func (p *ConfigParser) validateOutbounds(raw json.RawMessage) {
	var outbounds []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &outbounds); err != nil {
		return
	}
	for i, out := range outbounds {
		var outType string
		if t, ok := out["type"]; ok {
			json.Unmarshal(t, &outType)
		}
		switch outType {
		case "block":
			p.addError(DeprecatedField{
				Path:        fmt.Sprintf("outbounds[%d].type=block", i),
				Deprecated:  "1.11.0",
				Removed:     "1.13.0",
				Replacement: "rule action reject",
			})
		case "dns":
			p.addError(DeprecatedField{
				Path:        fmt.Sprintf("outbounds[%d].type=dns", i),
				Deprecated:  "1.11.0",
				Removed:     "1.13.0",
				Replacement: "rule action hijack-dns",
			})
		case "wireguard":
			p.addError(DeprecatedField{
				Path:        fmt.Sprintf("outbounds[%d].type=wireguard", i),
				Deprecated:  "1.11.0",
				Removed:     "1.13.0",
				Replacement: "endpoint (wireguard)",
			})
		case "direct":
			if _, ok := out["override_address"]; ok {
				p.addError(DeprecatedField{
					Path:        fmt.Sprintf("outbounds[%d].override_address", i),
					Deprecated:  "1.11.0",
					Removed:     "1.13.0",
					Replacement: "rule action override_address",
				})
			}
			if _, ok := out["override_port"]; ok {
				p.addError(DeprecatedField{
					Path:        fmt.Sprintf("outbounds[%d].override_port", i),
					Deprecated:  "1.11.0",
					Removed:     "1.13.0",
					Replacement: "rule action override_port",
				})
			}
		}
		// TLS inline ACME и legacy ECH
		if tlsRaw, ok := out["tls"]; ok {
			var tls map[string]json.RawMessage
			if err := json.Unmarshal(tlsRaw, &tls); err == nil {
				if _, ok := tls["acme"]; ok {
					p.addWarn(DeprecatedField{
						Path:        fmt.Sprintf("outbounds[%d].tls.acme", i),
						Deprecated:  "1.14.0",
						Removed:     "1.16.0",
						Replacement: "certificate provider (acme)",
					})
				}
				if _, ok := tls["pq_signature_schemes_enabled"]; ok {
					p.addError(DeprecatedField{
						Path:        fmt.Sprintf("outbounds[%d].tls.pq_signature_schemes_enabled", i),
						Deprecated:  "1.12.0",
						Removed:     "1.13.0",
						Replacement: "(removed, stdlib ECH)",
					})
				}
				if _, ok := tls["dynamic_record_sizing_disabled"]; ok {
					p.addError(DeprecatedField{
						Path:        fmt.Sprintf("outbounds[%d].tls.dynamic_record_sizing_disabled", i),
						Deprecated:  "1.12.0",
						Removed:     "1.13.0",
						Replacement: "(removed, unrelated to ECH)",
					})
				}
			}
		}
	}
}

func (p *ConfigParser) validateCacheFile(raw json.RawMessage) {
	var cf map[string]json.RawMessage
	if err := json.Unmarshal(raw, &cf); err != nil {
		return
	}
	if _, ok := cf["store_rdrc"]; ok {
		p.addWarn(DeprecatedField{
			Path:        "cache_file.store_rdrc",
			Deprecated:  "1.14.0",
			Removed:     "1.16.0",
			Replacement: "(removed, always enabled)",
		})
	}
}

func (p *ConfigParser) addWarn(d DeprecatedField) {
	p.result.Warnings = append(p.result.Warnings, d)
}

func (p *ConfigParser) addError(d DeprecatedField) {
	p.result.Errors = append(p.result.Errors, d)
}

// Config представляет распарсенный конфиг (обёртка над raw JSON)
type Config struct {
	raw map[string]json.RawMessage
}

// Raw возвращает raw JSON map
func (c *Config) Raw() map[string]json.RawMessage {
	return c.raw
}
