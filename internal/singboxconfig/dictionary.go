package singboxconfig

import (
	"fmt"
)

// Dictionary is a flattened index of schema paths.
type Dictionary struct {
	Fields map[string]*FieldInfo
}

// BuildDictionary creates a flat path -> FieldInfo index from a schema.
func BuildDictionary(s *Schema) (*Dictionary, error) {
	d := &Dictionary{Fields: make(map[string]*FieldInfo)}
	for name, node := range s.Fields {
		if err := d.walk(name, node); err != nil {
			return nil, err
		}
	}
	return d, nil
}

func (d *Dictionary) walk(path string, node *SchemaNode) error {
	info, err := nodeToFieldInfo(path, node)
	if err != nil {
		return err
	}
	d.Fields[path] = info

	switch node.Type {
	case "object":
		for name, child := range node.Children {
			if err := d.walk(path+"."+name, child); err != nil {
				return err
			}
		}
		for i := range node.OneOf {
			for name, child := range node.OneOf[i].Fields {
				if err := d.walk(path+"."+name, child); err != nil {
					return err
				}
			}
		}
	case "array":
		if node.Items != nil {
			if err := d.walk(path+"[]", node.Items); err != nil {
				return err
			}
		}
	}
	return nil
}

// Lookup returns the FieldInfo for a path, or nil if not found.
func (d *Dictionary) Lookup(path string) *FieldInfo {
	if d == nil {
		return nil
	}
	return d.Fields[path]
}

func nodeToFieldInfo(path string, node *SchemaNode) (*FieldInfo, error) {
	since, err := ParseVersion(node.Since)
	if err != nil {
		return nil, fmt.Errorf("%s: since: %w", path, err)
	}
	info := &FieldInfo{
		Path:        path,
		Since:       since,
		Replacement: node.Replacement,
		RenameTo:    node.RenameTo,
		Type:        node.Type,
		LegacyHint:  node.LegacyHint,
	}
	if node.Deprecated != "" {
		v, err := ParseVersion(node.Deprecated)
		if err != nil {
			return nil, fmt.Errorf("%s: deprecated: %w", path, err)
		}
		info.Deprecated = v
	}
	if node.Removed != "" {
		v, err := ParseVersion(node.Removed)
		if err != nil {
			return nil, fmt.Errorf("%s: removed: %w", path, err)
		}
		info.Removed = v
	}
	return info, nil
}
