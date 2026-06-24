package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// FieldDef captures a field parsed from a markdown file.
type FieldDef struct {
	Name        string
	Type        string // inferred type
	Since       string
	Deprecated  string
	Removed     string
	Replacement string
	RenameTo    string
	// Raw holds the description text for further heuristics.
	Raw string
}

// ChangeBlock captures a !!! quote "Changes in sing-box X.Y.Z" block.
type ChangeBlock struct {
	Version string
	Added   []string
	Changed []string // :material-alert:
	Removed []string // :material-delete-clock: or :material-delete-alert:
}

// ParsedDoc is the result of parsing a single markdown file.
type ParsedDoc struct {
	Title      string
	Raw        string
	Changes    []ChangeBlock
	Fields     map[string]*FieldDef
	Structure  map[string]any   // parsed JSON from the first ### Structure block
	Structures []map[string]any // all parsed JSON objects from the file
	SharedRefs []string         // e.g. "Listen Fields", "Dial Fields"
}

var (
	changesHeaderRE  = regexp.MustCompile(`!!!\s+quote\s+"Changes in sing-box ([0-9]+\.[0-9]+\.[0-9]+)"`)
	fieldHeadingRE   = regexp.MustCompile(`(?m)^####\s+([a-zA-Z0-9_]+)\s*$`)
	fieldSinceRE     = regexp.MustCompile(`(?mi)^!!!\s+question\s+"Since sing-box ([0-9]+\.[0-9]+\.[0-9]+)"`)
	fieldDeprecateRE = regexp.MustCompile(`(?mi)^!!!\s+(?:failure|warning|danger)\s+"Deprecated in sing-box ([0-9]+\.[0-9]+\.[0-9]+)"`)
	fieldRemovedRE   = regexp.MustCompile(`(?i)(?:removed\s+in|will\s+be\s+removed\s+in)\s+sing-box\s+([0-9]+\.[0-9]+\.[0-9]+)`)
	sharedRefRE      = regexp.MustCompile(`//\s*(Listen Fields|Dial Fields|TLS Fields|V2Ray Transport|Multiplex|UDP over TCP)`)
	materialItemRE   = regexp.MustCompile(`:\s*material-([a-z-]+):\s*\[([^\]]+)\]`)
	typeTableRowRE   = regexp.MustCompile(`(?m)^\|\s*([^|\n]+?)\s*\|\s*\[([^\]]+)\]\([^)]+\)\s*\|`)
)

// DefaultVariantFile parses the type table in an index document and returns the
// file name (e.g. "legacy") referenced by an "empty" or "default" row, if any.
func DefaultVariantFile(doc *ParsedDoc) string {
	for _, m := range typeTableRowRE.FindAllStringSubmatch(doc.Raw, -1) {
		typeCell := strings.ToLower(strings.TrimSpace(m[1]))
		typeCell = strings.ReplaceAll(typeCell, "`", "")
		if strings.Contains(typeCell, "empty") || strings.Contains(typeCell, "default") {
			return strings.ToLower(strings.TrimSpace(m[2]))
		}
	}
	return ""
}

// ParseMarkdown parses a sing-box configuration markdown file.
func ParseMarkdown(src string) (*ParsedDoc, error) {
	doc := &ParsedDoc{
		Raw:    src,
		Fields: make(map[string]*FieldDef),
	}

	lines := strings.Split(src, "\n")

	// Extract title (first # heading).
	for _, line := range lines {
		if strings.HasPrefix(line, "# ") {
			doc.Title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			break
		}
	}

	// Extract changes blocks.
	doc.Changes = parseChangesBlocks(src)

	// Extract structure JSON.
	doc.Structures = extractCodeBlocks(src, "json")
	if len(doc.Structures) > 0 {
		doc.Structure = doc.Structures[0]
	}

	// Extract shared schema references from comments.
	for _, m := range sharedRefRE.FindAllStringSubmatch(src, -1) {
		doc.SharedRefs = append(doc.SharedRefs, m[1])
	}

	// Extract field sections.
	matches := fieldHeadingRE.FindAllStringIndex(src, -1)
	for i, m := range matches {
		name := fieldHeadingRE.FindStringSubmatch(src[m[0]:m[1]])[1]
		start := m[1]
		end := len(src)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		text := src[start:end]
		fd := parseFieldDef(name, text)
		doc.Fields[name] = fd
	}

	doc.applyChanges()
	return doc, nil
}

// applyChanges propagates version metadata from Changes blocks into Fields.
func (doc *ParsedDoc) applyChanges() {
	for _, cb := range doc.Changes {
		for _, name := range cb.Added {
			if fd, ok := doc.Fields[name]; ok && fd.Since == "" {
				fd.Since = cb.Version
			}
		}
		for _, name := range cb.Changed {
			if fd, ok := doc.Fields[name]; ok && fd.Deprecated == "" {
				// :material-alert: usually marks a change, not deprecation.
				// Leave it unless text says otherwise.
			}
		}
		for _, name := range cb.Removed {
			fd, ok := doc.Fields[name]
			if !ok {
				fd = &FieldDef{Name: name}
				doc.Fields[name] = fd
			}
			if fd.Deprecated == "" {
				fd.Deprecated = cb.Version
			}
			if fd.Removed == "" {
				fd.Removed = nextMinor(cb.Version)
			}
			// Infer simple renames from a corresponding added field in the same block.
			if fd.RenameTo == "" {
				fd.RenameTo = findRename(name, cb.Added)
			}
		}
	}
}

// findRename returns the added field name that looks like a simple rename of old.
func findRename(old string, added []string) string {
	oldNorm := strings.ReplaceAll(old, "_", "")
	for _, a := range added {
		if strings.ReplaceAll(a, "_", "") == oldNorm {
			return a
		}
	}
	return ""
}

// nextMinor returns the next minor version string for a deprecation marker.
func nextMinor(v string) string {
	var major, minor, patch int
	fmt.Sscanf(v, "%d.%d.%d", &major, &minor, &patch)
	return fmt.Sprintf("%d.%d.0", major, minor+1)
}

func parseChangesBlocks(src string) []ChangeBlock {
	var blocks []ChangeBlock
	lines := strings.Split(src, "\n")
	var cur *ChangeBlock
	var inBlock bool
	for _, line := range lines {
		if m := changesHeaderRE.FindStringSubmatch(line); m != nil {
			if cur != nil {
				blocks = append(blocks, *cur)
			}
			cur = &ChangeBlock{Version: m[1]}
			inBlock = true
			continue
		}
		if !inBlock || cur == nil {
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		// Items are indented under the quote block.
		for _, mm := range materialItemRE.FindAllStringSubmatch(line, -1) {
			kind := strings.ToLower(strings.ReplaceAll(mm[1], "-", ""))
			ref := mm[2]
			// ref may be a markdown link anchor; keep just the field name.
			if idx := strings.Index(ref, "#"); idx != -1 {
				ref = ref[:idx]
			}
			ref = strings.TrimSpace(ref)
			switch kind {
			case "plus":
				cur.Added = append(cur.Added, ref)
			case "alert":
				cur.Changed = append(cur.Changed, ref)
			case "deleteclock", "deletealert":
				cur.Removed = append(cur.Removed, ref)
			}
		}
	}
	if cur != nil {
		blocks = append(blocks, *cur)
	}
	return blocks
}

func extractCodeBlock(src, lang string) string {
	re := regexp.MustCompile("(?s)```" + regexp.QuoteMeta(lang) + "\\s*(.*?)```")
	m := re.FindStringSubmatch(src)
	if m == nil {
		return ""
	}
	return sanitizeJSONExample(m[1])
}

func extractCodeBlocks(src, lang string) []map[string]any {
	re := regexp.MustCompile("(?s)```" + regexp.QuoteMeta(lang) + "\\s*(.*?)```")
	var out []map[string]any
	for _, m := range re.FindAllStringSubmatch(src, -1) {
		block := sanitizeJSONExample(m[1])
		var v map[string]any
		if err := json.Unmarshal([]byte(block), &v); err == nil {
			out = append(out, v)
		}
	}
	return out
}

// stripJSONComments removes // comments, /* */ comments and doc placeholders
// (like ...) from JSON text so that examples can be parsed.
func stripJSONComments(src string) string {
	var out strings.Builder
	for i := 0; i < len(src); {
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '/' {
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		}
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '*' {
			i += 2
			for i < len(src)-1 && !(src[i] == '*' && src[i+1] == '/') {
				i++
			}
			i += 2
			continue
		}
		out.WriteByte(src[i])
		i++
	}
	return out.String()
}

// sanitizeJSONExample cleans up documentation JSON so it can be parsed.
func sanitizeJSONExample(src string) string {
	src = stripJSONComments(src)
	// Remove doc placeholders like ... // Foo or standalone ...
	re := regexp.MustCompile(`\.\.\..*`)
	src = re.ReplaceAllString(src, "")
	// Remove trailing commas before closing braces/brackets.
	re = regexp.MustCompile(`,(\s*[}\]])`)
	src = re.ReplaceAllString(src, "$1")
	return src
}

func parseFieldDef(name, text string) *FieldDef {
	fd := &FieldDef{Name: name, Raw: text}
	if m := fieldSinceRE.FindStringSubmatch(text); m != nil {
		fd.Since = m[1]
	}
	if m := fieldDeprecateRE.FindStringSubmatch(text); m != nil {
		fd.Deprecated = m[1]
	}
	// If deprecated text mentions a replacement field, record it.
	if fd.Deprecated != "" {
		fd.Replacement = extractReplacement(text)
		if fd.Replacement != "" && !strings.Contains(fd.Replacement, ".") {
			fd.RenameTo = fd.Replacement
		}
		// Only set removed if the text actually refers to this field.
		if fd.Removed == "" && strings.Contains(text, fd.Name) {
			if m := fieldRemovedRE.FindStringSubmatch(text); m != nil {
				fd.Removed = m[1]
			}
		}
	}
	return fd
}

var replacementRE = regexp.MustCompile("(?i)(?:replaced?\\s+(?:by|with)|renamed?\\s+(?:to|into)|use|migrated?\\s+(?:to|into)|merged?\\s+(?:to|into))\\s+['\"`]?([a-z0-9_\\.\\[\\]]+)['\"`]?")

func extractReplacement(text string) string {
	for _, m := range replacementRE.FindAllStringSubmatch(text, -1) {
		r := m[1]
		if r != "" {
			return r
		}
	}
	return ""
}

// inferTypeFromName guesses a YAML/JSON type from a field name.
func inferTypeFromName(name string) string {
	switch {
	case strings.HasSuffix(name, "_enabled") || name == "enabled" ||
		strings.HasSuffix(name, "_disabled") || name == "disabled":
		return "boolean"
	case name == "port" || strings.HasSuffix(name, "_port") || name == "mtu" ||
		strings.HasSuffix(name, "_count") || strings.HasSuffix(name, "_capacity") ||
		strings.HasSuffix(name, "_index") || strings.HasSuffix(name, "_size") ||
		name == "user_id" || strings.HasSuffix(name, "_id"):
		return "integer"
	case strings.HasSuffix(name, "_timeout") || strings.HasSuffix(name, "_interval") ||
		strings.HasSuffix(name, "_delay") || strings.HasSuffix(name, "_ttl"):
		return "duration"
	case name == "type" || name == "tag" || strings.HasSuffix(name, "_address") ||
		strings.HasSuffix(name, "_path") || strings.HasSuffix(name, "_strategy") ||
		strings.HasSuffix(name, "_mode") || strings.HasSuffix(name, "_interface") ||
		strings.HasSuffix(name, "_domain") || strings.HasSuffix(name, "_package") ||
		strings.HasSuffix(name, "_uid") || strings.HasSuffix(name, "_ssid") ||
		strings.HasSuffix(name, "_bssid") || strings.HasSuffix(name, "_mark"):
		return "string"
	}
	return "any"
}

// inferTypeFromValue guesses a type from a JSON value.
func inferTypeFromValue(v any) string {
	switch x := v.(type) {
	case nil:
		return "any"
	case bool:
		return "boolean"
	case float64:
		if x == float64(int64(x)) {
			return "integer"
		}
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return "any"
}

// inferFieldType determines the best type for a field using its definition and
// the structure example.
func inferFieldType(fd *FieldDef, example any) string {
	if fd.Type != "" {
		return fd.Type
	}
	if example != nil {
		t := inferTypeFromValue(example)
		if t != "any" {
			// If structure says array but name suggests scalar, prefer scalar
			// because examples often wrap single values in arrays.
			if t == "array" {
				if arr, ok := example.([]any); ok && len(arr) > 0 {
					itemType := inferTypeFromValue(arr[0])
					if itemType == "string" {
						return "string"
					}
					if itemType == "integer" {
						return "integer"
					}
					if itemType == "boolean" {
						return "boolean"
					}
				}
				return "string" // common default
			}
			return t
		}
	}
	return inferTypeFromName(fd.Name)
}

// parseVersion parses a version string for comparisons.
func parseVersion(s string) VersionTag {
	var v VersionTag
	fmt.Sscanf(s, "%d.%d.%d", &v.Major, &v.Minor, &v.Patch)
	return v
}

// earlier returns the earlier of two version strings.
func earlier(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	va, vb := parseVersion(a), parseVersion(b)
	if va.Less(vb) {
		return a
	}
	return b
}

// later returns the later of two version strings.
func later(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	va, vb := parseVersion(a), parseVersion(b)
	if vb.Less(va) {
		return a
	}
	return b
}

// boolStr parses a string as a boolean, returning a pointer.
func boolStr(s string) *bool {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return nil
	}
	return &v
}
