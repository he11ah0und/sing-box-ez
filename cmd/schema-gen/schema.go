package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"sing-box-ez/internal/singboxconfig"
)

// SchemaBuilder incrementally constructs a schema tree from parsed docs.
type SchemaBuilder struct {
	Repo     *Repo
	Tags     []VersionTag
	Earliest string
	Latest   string
	Fields   map[string]*BuilderNode
	Shared   map[string]*BuilderNode
}

type BuilderNode struct {
	Name                 string
	Type                 string
	Since                string
	Deprecated           string
	Removed              string
	Replacement          string
	RenameTo             string
	Children             map[string]*BuilderNode
	Items                *BuilderNode
	OneOfBy              string
	OneOf                []BuilderVariant
	AdditionalProperties bool
}

type BuilderVariant struct {
	When   map[string]string
	Fields map[string]*BuilderNode
}

func NewSchemaBuilder(repo *Repo, tags []VersionTag) *SchemaBuilder {
	earliest := "1.0.0"
	return &SchemaBuilder{
		Repo:     repo,
		Tags:     tags,
		Earliest: earliest,
		Latest:   tags[len(tags)-1].String(),
		Fields:   make(map[string]*BuilderNode),
		Shared:   make(map[string]*BuilderNode),
	}
}

func (b *SchemaBuilder) Build() (*singboxconfig.Schema, error) {
	latest := b.Tags[len(b.Tags)-1].Tag

	// Load shared schemas first.
	if err := b.loadShared(latest); err != nil {
		return nil, fmt.Errorf("load shared schemas: %w", err)
	}

	// Load top-level configuration sections from the latest tag.
	sections := []struct {
		file string
		name string
	}{
		{"docs/configuration/log/index.md", "log"},
		{"docs/configuration/ntp/index.md", "ntp"},
		{"docs/configuration/certificate/index.md", "certificate"},
		{"docs/configuration/dns/index.md", "dns"},
		{"docs/configuration/route/index.md", "route"},
		{"docs/configuration/experimental/index.md", "experimental"},
		{"docs/configuration/inbounds.md", "inbounds"},
		{"docs/configuration/outbounds.md", "outbounds"},
		{"docs/configuration/endpoints.md", "endpoints"},
		{"docs/configuration/services.md", "services"},
	}
	for _, sec := range sections {
		if !b.Repo.FileExistsAt(latest, sec.file) {
			// Try older file naming.
			alt := strings.Replace(sec.file, "/index.md", ".md", 1)
			if b.Repo.FileExistsAt(latest, alt) {
				sec.file = alt
			}
		}
		if !b.Repo.FileExistsAt(latest, sec.file) {
			continue
		}
		data, err := b.Repo.ReadFileAt(latest, sec.file)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", sec.file, err)
		}
		doc, err := ParseMarkdown(string(data))
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", sec.file, err)
		}
		node := b.objectFromDoc(doc, sec.name, mainStructure(doc, sec.name))
		b.Fields[sec.name] = node
	}

	// Typed arrays: inbounds, outbounds, endpoints, dns.servers, services.
	b.buildTypedArray(latest, "inbounds", "docs/configuration/inbound", "")
	b.buildTypedArray(latest, "outbounds", "docs/configuration/outbound", "")
	b.buildTypedArray(latest, "endpoints", "docs/configuration/endpoint", "")
	b.buildTypedArray(latest, "dns.servers", "docs/configuration/dns/server", "dns")
	b.buildServices(latest)

	// Nested objects referenced from docs.
	b.buildNested(latest, "dns.fakeip", "docs/configuration/dns/fakeip.md")
	// DNS and route rules.
	b.buildRuleItems(latest, "dns.rules", "docs/configuration/dns/rule.md")
	b.buildRuleItems(latest, "route.rules", "docs/configuration/route/rule.md")

	b.buildNested(latest, "experimental.cache_file", "docs/configuration/experimental/cache-file.md")
	b.buildNested(latest, "experimental.clash_api", "docs/configuration/experimental/clash-api.md")
	b.buildNested(latest, "experimental.v2ray_api", "docs/configuration/experimental/v2ray-api.md")

	// Convert builder tree to output schema.
	out := &singboxconfig.Schema{
		Version:       "1.0.0",
		SingboxLatest: b.Latest,
		Fields:        make(map[string]*singboxconfig.SchemaNode),
	}
	for name, node := range b.Fields {
		out.Fields[name] = b.toSchemaNode(node)
	}
	return out, nil
}

func (b *SchemaBuilder) loadShared(tag string) error {
	prefix := "docs/configuration/shared"
	files, err := b.Repo.ListFilesAt(tag, prefix)
	if err != nil {
		return nil // shared may not exist in old versions
	}
	for _, f := range files {
		if !strings.HasSuffix(f, ".md") || strings.HasSuffix(f, ".zh.md") {
			continue
		}
		data, err := b.Repo.ReadFileAt(tag, f)
		if err != nil {
			return err
		}
		doc, err := ParseMarkdown(string(data))
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(filepath.Base(f), ".md")
		b.Shared[name] = b.objectFromDoc(doc, name, mergeStructures(doc.Structures))
	}
	return nil
}

// mergeStructures deep-merges all JSON structure examples from a markdown file.
// It is used for shared schemas (e.g. TLS, V2Ray Transport) that document
// different contexts (inbound/outbound, client/server) in separate code blocks.
func mergeStructures(structures []map[string]any) map[string]any {
	out := make(map[string]any)
	for _, s := range structures {
		for k, v := range s {
			if existing, ok := out[k]; ok {
				if existingMap, ok1 := existing.(map[string]any); ok1 {
					if vMap, ok2 := v.(map[string]any); ok2 {
						out[k] = mergeStructures([]map[string]any{existingMap, vMap})
						continue
					}
				}
			}
			out[k] = v
		}
	}
	return out
}

// objectFromDoc builds a BuilderNode of type object from a parsed doc using the
// provided structure value (the object this file documents).
func (b *SchemaBuilder) objectFromDoc(doc *ParsedDoc, name string, structVal any) *BuilderNode {
	node := &BuilderNode{
		Name:     name,
		Type:     "object",
		Children: make(map[string]*BuilderNode),
		Since:    b.Earliest,
	}
	// First expand the full structure example so nested objects are present.
	if m, ok := structVal.(map[string]any); ok {
		for k, v := range m {
			b.setChild(node, k, b.valueToNode(k, v))
		}
	}
	// Then apply documented field definitions, resolving dotted paths in the example.
	for _, fd := range doc.Fields {
		example := resolvePath(structVal, fd.Name)
		child := b.fieldDefToNode(fd, example)
		if child != nil {
			b.setChild(node, fd.Name, child)
		}
	}
	return node
}

// resolvePath walks a dotted path through nested map[string]any structures.
func resolvePath(root any, path string) any {
	parts := strings.Split(path, ".")
	v := root
	for _, p := range parts {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[p]
	}
	return v
}

// nodeFromFields wraps a fields map in a temporary BuilderNode so setChild can
// populate it.
func nodeFromFields(fields map[string]*BuilderNode) *BuilderNode {
	return &BuilderNode{Children: fields}
}

// setChild adds a child to a node, interpreting dotted names as nested paths.
func (b *SchemaBuilder) setChild(parent *BuilderNode, name string, child *BuilderNode) {
	parts := strings.Split(name, ".")
	node := parent
	for i := 0; i < len(parts)-1; i++ {
		if node.Children == nil {
			node.Children = make(map[string]*BuilderNode)
		}
		next, ok := node.Children[parts[i]]
		if !ok {
			next = &BuilderNode{Name: parts[i], Type: "object", Since: b.Earliest, Children: make(map[string]*BuilderNode)}
			node.Children[parts[i]] = next
		}
		if next.Type == "" {
			next.Type = "object"
		}
		node = next
	}
	if node.Children == nil {
		node.Children = make(map[string]*BuilderNode)
	}
	node.Children[parts[len(parts)-1]] = child
}

// mainStructure extracts the primary object documented by a markdown file.
func mainStructure(doc *ParsedDoc, name string) any {
	if name != "" {
		if v, ok := doc.Structure[name]; ok {
			return v
		}
	}
	// If the structure root has a single key, return its value.
	if len(doc.Structure) == 1 {
		for _, v := range doc.Structure {
			return v
		}
	}
	return doc.Structure
}

func (b *SchemaBuilder) fieldDefToNode(fd *FieldDef, example any) *BuilderNode {
	since := fd.Since
	if since == "" {
		since = b.Earliest
	}
	node := &BuilderNode{
		Name:        fd.Name,
		Type:        inferFieldType(fd, example),
		Since:       since,
		Deprecated:  fd.Deprecated,
		Removed:     fd.Removed,
		Replacement: fd.Replacement,
		RenameTo:    fd.RenameTo,
		Children:    make(map[string]*BuilderNode),
	}
	if node.Type == "object" && node.Name == "headers" {
		node.AdditionalProperties = true
	}
	// Expand structure example for object/array children.
	if node.Type == "object" {
		if m, ok := example.(map[string]any); ok {
			for k, v := range m {
				node.Children[k] = b.valueToNode(k, v)
			}
		}
	} else if node.Type == "array" {
		if arr, ok := example.([]any); ok && len(arr) > 0 {
			node.Items = b.valueToNode(fd.Name+"[]", arr[0])
		} else {
			node.Items = &BuilderNode{Type: "any"}
		}
	}
	return node
}

func (b *SchemaBuilder) valueToNode(name string, v any) *BuilderNode {
	t := inferTypeFromValue(v)
	node := &BuilderNode{Name: name, Type: t, Since: b.Earliest, Children: make(map[string]*BuilderNode)}
	if t == "object" && name == "headers" {
		node.AdditionalProperties = true
	}
	switch t {
	case "object":
		if m, ok := v.(map[string]any); ok {
			for k, val := range m {
				node.Children[k] = b.valueToNode(k, val)
			}
		}
	case "array":
		if arr, ok := v.([]any); ok && len(arr) > 0 {
			node.Items = b.valueToNode(name+"[]", arr[0])
		} else {
			node.Items = &BuilderNode{Name: name + "[]", Type: "any", Since: b.Earliest}
		}
	}
	return node
}

// buildTypedArray assembles a root array field whose items are discriminated by `type`.
func (b *SchemaBuilder) buildTypedArray(tag, rootPath, dir, parentField string) {
	rootName := rootPath
	if idx := strings.Index(rootPath, "."); idx != -1 {
		rootName = rootPath[:idx]
	}
	indexPath := dir + "/index.md"
	if !b.Repo.FileExistsAt(tag, indexPath) {
		return
	}
	data, _ := b.Repo.ReadFileAt(tag, indexPath)
	doc, _ := ParseMarkdown(string(data))

	items := &BuilderNode{
		Type:    "object",
		Since:   b.Earliest,
		OneOfBy: "type",
	}

	// Default variant with common fields. If the type table names a default
	// variant file (e.g. "legacy" for DNS servers), use that file.
	common := make(map[string]*BuilderNode)
	common["type"] = &BuilderNode{Name: "type", Type: "string", Since: b.Earliest}
	defaultFile := DefaultVariantFile(doc)
	if defaultFile != "" {
		if data, err := b.Repo.ReadFileAt(tag, dir+"/"+defaultFile+".md"); err == nil {
			if defaultDoc, err := ParseMarkdown(string(data)); err == nil {
				doc = defaultDoc
			}
		}
	}
	for _, fd := range doc.Fields {
		b.setChild(nodeFromFields(common), fd.Name, b.fieldDefToNode(fd, doc.Structure[fd.Name]))
	}
	// Inline shared listen/dial fields based on the context.
	for _, ref := range doc.SharedRefs {
		b.inlineShared(common, ref)
	}
	if rootName == "inbounds" {
		b.inlineShared(common, "Listen Fields")
	} else if rootName == "outbounds" || rootName == "dns" {
		b.inlineShared(common, "Dial Fields")
	}
	b.inlineSharedByFieldName(common)
	items.OneOf = append(items.OneOf, BuilderVariant{When: map[string]string{}, Fields: common})

	// Discover typed files.
	files, err := b.Repo.ListFilesAt(tag, dir)
	if err != nil {
		return
	}
	for _, f := range files {
		if !strings.HasSuffix(f, ".md") || strings.HasSuffix(f, ".zh.md") || filepath.Base(f) == "index.md" {
			continue
		}
		variant := strings.TrimSuffix(filepath.Base(f), ".md")
		if variant == "" || variant == defaultFile {
			continue
		}
		data, err := b.Repo.ReadFileAt(tag, f)
		if err != nil {
			continue
		}
		doc, err := ParseMarkdown(string(data))
		if err != nil {
			continue
		}
		fields := make(map[string]*BuilderNode)
		fields["type"] = &BuilderNode{Name: "type", Type: "string", Since: b.Earliest}
		mainStruct := mainStructure(doc, "")
		for _, fd := range doc.Fields {
			var example any
			if m, ok := mainStruct.(map[string]any); ok {
				example = m[fd.Name]
			}
			fields[fd.Name] = b.fieldDefToNode(fd, example)
		}
		for _, ref := range doc.SharedRefs {
			b.inlineShared(fields, ref)
		}
		b.inlineSharedByFieldName(fields)
		items.OneOf = append(items.OneOf, BuilderVariant{
			When:   map[string]string{"type": variant},
			Fields: fields,
		})
	}

	// Place the array into the schema tree.
	if rootName == rootPath {
		b.Fields[rootName] = &BuilderNode{
			Name:  rootName,
			Type:  "array",
			Since: b.Earliest,
			Items: items,
		}
	} else {
		// e.g. dns.servers
		parts := strings.Split(rootPath, ".")
		parent := b.Fields[parts[0]]
		if parent == nil {
			return
		}
		parent.Children[parts[1]] = &BuilderNode{
			Name:  parts[1],
			Type:  "array",
			Since: b.Earliest,
			Items: items,
		}
	}
}

func (b *SchemaBuilder) buildServices(tag string) {
	prefix := "docs/configuration/services"
	if !b.Repo.FileExistsAt(tag, prefix+"/index.md") {
		return
	}
	items := &BuilderNode{Type: "object", Since: b.Earliest, OneOfBy: "type"}
	files, _ := b.Repo.ListFilesAt(tag, prefix)
	for _, f := range files {
		if !strings.HasSuffix(f, ".md") || strings.HasSuffix(f, ".zh.md") || filepath.Base(f) == "index.md" {
			continue
		}
		variant := strings.TrimSuffix(filepath.Base(f), ".md")
		data, _ := b.Repo.ReadFileAt(tag, f)
		doc, _ := ParseMarkdown(string(data))
		fields := make(map[string]*BuilderNode)
		fields["type"] = &BuilderNode{Name: "type", Type: "string", Since: b.Earliest}
		for _, fd := range doc.Fields {
			b.setChild(nodeFromFields(fields), fd.Name, b.fieldDefToNode(fd, nil))
		}
		b.inlineSharedByFieldName(fields)
		items.OneOf = append(items.OneOf, BuilderVariant{When: map[string]string{"type": variant}, Fields: fields})
	}
	b.Fields["services"] = &BuilderNode{Name: "services", Type: "array", Since: b.Earliest, Items: items}
}

// buildRuleItems parses a *_rule.md file and creates an array field with
// default and logical variants under the given parent.field path.
func (b *SchemaBuilder) buildRuleItems(tag, fullPath, file string) {
	if !b.Repo.FileExistsAt(tag, file) {
		return
	}
	data, _ := b.Repo.ReadFileAt(tag, file)
	doc, _ := ParseMarkdown(string(data))

	parts := strings.Split(fullPath, ".")
	parentName, fieldName := parts[0], parts[1]
	parent := b.Fields[parentName]
	if parent == nil {
		return
	}

	items := &BuilderNode{Type: "object", Since: b.Earliest, OneOfBy: "type"}

	// Default variant (empty type): fields from parsed doc.
	defaultFields := make(map[string]*BuilderNode)
	defaultFields["type"] = &BuilderNode{Name: "type", Type: "string", Since: b.Earliest}
	for _, fd := range doc.Fields {
		b.setChild(nodeFromFields(defaultFields), fd.Name, b.fieldDefToNode(fd, nil))
	}
	b.inlineSharedByFieldName(defaultFields)
	items.OneOf = append(items.OneOf, BuilderVariant{When: map[string]string{}, Fields: defaultFields})

	// Logical variant if the file documents it.
	if _, ok := findLogicalRule(doc); ok {
		logicalFields := make(map[string]*BuilderNode)
		logicalFields["type"] = &BuilderNode{Name: "type", Type: "string", Since: b.Earliest}
		logicalFields["mode"] = &BuilderNode{Name: "mode", Type: "string", Since: b.Earliest}
		logicalFields["rules"] = &BuilderNode{Name: "rules", Type: "array", Since: b.Earliest, Items: &BuilderNode{Type: "object", Since: b.Earliest}}
		// Copy action/server/outbound etc from default variant.
		for k, v := range defaultFields {
			if k != "type" {
				logicalFields[k] = v
			}
		}
		items.OneOf = append(items.OneOf, BuilderVariant{When: map[string]string{"type": "logical"}, Fields: logicalFields})
	}

	parent.Children[fieldName] = &BuilderNode{Name: fieldName, Type: "array", Since: b.Earliest, Items: items}
}

// findLogicalRule returns a logical rule prototype from the structure example.
func findLogicalRule(doc *ParsedDoc) (map[string]any, bool) {
	var walk func(any) (map[string]any, bool)
	walk = func(v any) (map[string]any, bool) {
		switch x := v.(type) {
		case map[string]any:
			if t, _ := x["type"].(string); t == "logical" {
				return x, true
			}
			for _, val := range x {
				if r, ok := walk(val); ok {
					return r, true
				}
			}
		case []any:
			for _, val := range x {
				if r, ok := walk(val); ok {
					return r, true
				}
			}
		}
		return nil, false
	}
	return walk(doc.Structure)
}

func (b *SchemaBuilder) buildNested(tag, path, file string) {
	if !b.Repo.FileExistsAt(tag, file) {
		return
	}
	data, _ := b.Repo.ReadFileAt(tag, file)
	doc, _ := ParseMarkdown(string(data))
	node := b.objectFromDoc(doc, path, mainStructure(doc, ""))
	parts := strings.Split(path, ".")
	parent := b.Fields[parts[0]]
	for i := 1; i < len(parts)-1; i++ {
		if parent == nil {
			return
		}
		parent = parent.Children[parts[i]]
	}
	if parent == nil {
		return
	}
	parent.Children[parts[len(parts)-1]] = node
}

func (b *SchemaBuilder) inlineShared(fields map[string]*BuilderNode, ref string) {
	key := sharedKey(ref)
	shared, ok := b.Shared[key]
	if !ok {
		return
	}
	for k, v := range shared.Children {
		if _, exists := fields[k]; !exists {
			fields[k] = v
		}
	}
}

var sharedFieldAliases = map[string]string{
	"transport": "v2ray-transport",
}

func (b *SchemaBuilder) inlineSharedByFieldName(fields map[string]*BuilderNode) {
	for name, node := range fields {
		if node.Type != "object" {
			continue
		}
		keys := []string{strings.ReplaceAll(name, "_", "-")}
		if alias, ok := sharedFieldAliases[name]; ok {
			keys = append(keys, alias)
		}
		var shared *BuilderNode
		for _, key := range keys {
			if s, ok := b.Shared[key]; ok {
				shared = s
				break
			}
		}
		if shared == nil {
			continue
		}
		if len(node.Children) == 0 {
			node.Children = make(map[string]*BuilderNode)
		}
		for k, v := range shared.Children {
			if _, exists := node.Children[k]; !exists {
				node.Children[k] = v
			}
		}
	}
}

func sharedKey(ref string) string {
	ref = strings.ToLower(strings.TrimSpace(ref))
	ref = strings.ReplaceAll(ref, " fields", "")
	ref = strings.ReplaceAll(ref, " ", "-")
	return ref
}

func (b *SchemaBuilder) toSchemaNode(n *BuilderNode) *singboxconfig.SchemaNode {
	if n == nil {
		return nil
	}
	node := &singboxconfig.SchemaNode{
		Since:                n.Since,
		Deprecated:           n.Deprecated,
		Removed:              n.Removed,
		Replacement:          n.Replacement,
		RenameTo:             n.RenameTo,
		Type:                 n.Type,
		AdditionalProperties: n.AdditionalProperties,
		OneOfBy:              n.OneOfBy,
	}
	if len(n.Children) > 0 {
		node.Children = make(map[string]*singboxconfig.SchemaNode)
		for k, v := range n.Children {
			node.Children[k] = b.toSchemaNode(v)
		}
	}
	if n.Items != nil {
		node.Items = b.toSchemaNode(n.Items)
	}
	for _, v := range n.OneOf {
		node.OneOf = append(node.OneOf, singboxconfig.TypedVariant{
			When:   v.When,
			Fields: b.toSchemaChildren(v.Fields),
		})
	}
	return node
}

func (b *SchemaBuilder) toSchemaChildren(m map[string]*BuilderNode) map[string]*singboxconfig.SchemaNode {
	out := make(map[string]*singboxconfig.SchemaNode)
	for k, v := range m {
		out[k] = b.toSchemaNode(v)
	}
	return out
}

// WriteYAML writes the schema to path as YAML.
func WriteYAML(s *singboxconfig.Schema, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString("# Generated by cmd/schema-gen from the official sing-box documentation.\n# DO NOT EDIT MANUALLY.\n\n"); err != nil {
		return err
	}
	enc := yaml.NewEncoder(f)
	enc.SetIndent(2)
	if err := enc.Encode(s); err != nil {
		return err
	}
	return enc.Close()
}
