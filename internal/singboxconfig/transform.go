package singboxconfig

import (
	"encoding/json"
	"fmt"
)

// Transform converts a config from one sing-box version to another.
// It removes fields that are removed in the target version and renames
// fields that have a rename_to attribute and are deprecated in the target
// version. Complex structural migrations are not handled automatically.
func Transform(input []byte, from, to string) ([]byte, error) {
	fromV, err := ParseVersion(from)
	if err != nil {
		return nil, fmt.Errorf("from version: %w", err)
	}
	toV, err := ParseVersion(to)
	if err != nil {
		return nil, fmt.Errorf("to version: %w", err)
	}

	var tree map[string]any
	if err := json.Unmarshal(input, &tree); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	transformObject(loadedSchema.Fields, fromV, toV, tree)

	out, err := json.MarshalIndent(tree, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	return out, nil
}

func transformObject(schema map[string]*SchemaNode, from, to Version, obj map[string]any) {
	for key, node := range schema {
		if node == nil {
			continue
		}
		val, ok := obj[key]
		if !ok {
			continue
		}

		if shouldRename(node, from, to) {
			delete(obj, key)
			obj[node.RenameTo] = val
			continue
		}

		if shouldRemove(node, from, to) {
			delete(obj, key)
			continue
		}

		transformValue(node, from, to, val)
	}
}

func transformValue(node *SchemaNode, from, to Version, v any) {
	switch node.Type {
	case "object":
		obj, ok := v.(map[string]any)
		if !ok {
			return
		}
		fields := node.Children
		if node.OneOfBy != "" && len(node.OneOf) > 0 {
			fields = resolveVariantFields(node, obj)
		}
		transformObject(fields, from, to, obj)
	case "array":
		arr, ok := v.([]any)
		if !ok {
			return
		}
		if node.Items == nil {
			return
		}
		for _, item := range arr {
			transformValue(node.Items, from, to, item)
		}
	}
}

func resolveVariantFields(node *SchemaNode, obj map[string]any) map[string]*SchemaNode {
	fields := make(map[string]*SchemaNode)
	for k, v := range node.Children {
		fields[k] = v
	}

	discriminator := ""
	if raw, ok := obj[node.OneOfBy]; ok {
		switch s := raw.(type) {
		case string:
			discriminator = s
		}
	}

	matched := false
	for _, variant := range node.OneOf {
		if len(variant.When) == 0 {
			continue
		}
		match := true
		for k, want := range variant.When {
			if k != node.OneOfBy || discriminator != want {
				match = false
				break
			}
		}
		if match {
			for k, v := range variant.Fields {
				fields[k] = v
			}
			matched = true
			break
		}
	}
	if !matched {
		for _, variant := range node.OneOf {
			if len(variant.When) == 0 {
				for k, v := range variant.Fields {
					fields[k] = v
				}
				break
			}
		}
	}
	return fields
}

func shouldRemove(node *SchemaNode, from, to Version) bool {
	if node.RemovedV.Equal(Version{}) {
		return false
	}
	return node.RemovedV.LessOrEqual(to)
}

func shouldRename(node *SchemaNode, from, to Version) bool {
	if node.RenameTo == "" {
		return false
	}
	if !node.DeprecatedV.Equal(Version{}) {
		return node.DeprecatedV.LessOrEqual(to)
	}
	return node.SinceV.LessOrEqual(to)
}
