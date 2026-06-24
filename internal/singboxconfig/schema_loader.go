package singboxconfig

import (
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadSchema reads a schema from a YAML reader and validates it.
func LoadSchema(r io.Reader) (*Schema, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read schema: %w", err)
	}
	var s Schema
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("unmarshal schema: %w", err)
	}
	normalizeSchema(&s)
	if err := validateSchema(&s); err != nil {
		return nil, fmt.Errorf("invalid schema: %w", err)
	}
	return &s, nil
}

func mustParseVersion(s string) Version {
	v, err := ParseVersion(s)
	if err != nil {
		panic(fmt.Sprintf("singboxconfig: invalid version %q: %v", s, err))
	}
	return v
}

// normalizeSchema fills missing 'since' on pure container nodes from their parents.
func normalizeSchema(s *Schema) {
	for _, node := range s.Fields {
		normalizeNode(node, node.Since)
	}
}

func normalizeNode(node *SchemaNode, parentSince string) {
	if node == nil {
		return
	}
	if node.Since == "" {
		node.Since = parentSince
	}
	node.SinceV = mustParseVersion(node.Since)
	if node.Deprecated != "" {
		node.DeprecatedV = mustParseVersion(node.Deprecated)
	}
	if node.Removed != "" {
		node.RemovedV = mustParseVersion(node.Removed)
	}
	for _, child := range node.Children {
		normalizeNode(child, node.Since)
	}
	if node.Items != nil {
		normalizeNode(node.Items, node.Since)
	}
	for i := range node.OneOf {
		for _, f := range node.OneOf[i].Fields {
			normalizeNode(f, node.Since)
		}
	}
}

func validateSchema(s *Schema) error {
	if strings.TrimSpace(s.Version) == "" {
		return fmt.Errorf("schema version is required")
	}
	if _, err := ParseVersion(s.Version); err != nil {
		return fmt.Errorf("schema version: %w", err)
	}
	if s.SingboxLatest == "" {
		return fmt.Errorf("singbox_latest is required")
	}
	if _, err := ParseVersion(s.SingboxLatest); err != nil {
		return fmt.Errorf("singbox_latest: %w", err)
	}
	if len(s.Fields) == 0 {
		return fmt.Errorf("schema has no fields")
	}
	seen := make(map[string]struct{})
	for name, node := range s.Fields {
		if err := validateNode(name, node, seen); err != nil {
			return err
		}
	}
	return nil
}

func validateNode(path string, node *SchemaNode, seen map[string]struct{}) error {
	if node == nil {
		return fmt.Errorf("%s: nil node", path)
	}
	if _, ok := seen[path]; ok {
		return fmt.Errorf("%s: duplicate path", path)
	}
	seen[path] = struct{}{}

	node.Type = strings.TrimSpace(node.Type)
	if node.Type == "" {
		return fmt.Errorf("%s: type is required", path)
	}
	validTypes := map[string]struct{}{
		"object":   {},
		"array":    {},
		"string":   {},
		"integer":  {},
		"boolean":  {},
		"number":   {},
		"duration": {},
		"enum":     {},
		"any":      {},
	}
	if _, ok := validTypes[node.Type]; !ok {
		return fmt.Errorf("%s: unknown type %q", path, node.Type)
	}

	if node.Since == "" {
		return fmt.Errorf("%s: since is required", path)
	}
	if _, err := ParseVersion(node.Since); err != nil {
		return fmt.Errorf("%s: invalid since %q: %w", path, node.Since, err)
	}
	for _, v := range []string{node.Deprecated, node.Removed} {
		if v != "" {
			if _, err := ParseVersion(v); err != nil {
				return fmt.Errorf("%s: invalid version %q: %w", path, v, err)
			}
		}
	}
	if node.Removed != "" && node.Deprecated == "" {
		return fmt.Errorf("%s: removed requires deprecated", path)
	}

	switch node.Type {
	case "object":
		if node.Items != nil {
			return fmt.Errorf("%s: object cannot have items", path)
		}
		for name, child := range node.Children {
			childPath := path + "." + name
			if err := validateNode(childPath, child, seen); err != nil {
				return err
			}
		}
		for i := range node.OneOf {
			for name, child := range node.OneOf[i].Fields {
				childPath := fmt.Sprintf("%s.<one_of[%d]>.%s", path, i, name)
				if err := validateNode(childPath, child, seen); err != nil {
					return err
				}
			}
		}
	case "array":
		if node.Children != nil {
			return fmt.Errorf("%s: array cannot have children", path)
		}
		if node.Items == nil {
			return fmt.Errorf("%s: array requires items", path)
		}
		if err := validateNode(path+"[]", node.Items, seen); err != nil {
			return err
		}
	}
	return nil
}
