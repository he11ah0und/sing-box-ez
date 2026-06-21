// Package yaml provides helpers for working with nested YAML trees.
package yaml

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// LoadTree parses YAML data into a nested map.
func LoadTree(data []byte) (map[string]any, error) {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal yaml: %w", err)
	}
	if raw == nil {
		return make(map[string]any), nil
	}
	return raw, nil
}

// GetPath returns a value from a nested map by path segments.
func GetPath(tree map[string]any, path ...string) (any, bool) {
	if len(path) == 0 || tree == nil {
		return nil, false
	}
	current, ok := tree[path[0]]
	if !ok {
		return nil, false
	}
	for _, p := range path[1:] {
		sub, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = sub[p]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// SetPath sets a value in a nested map, creating intermediate maps as needed.
func SetPath(tree map[string]any, value any, path ...string) {
	if len(path) == 0 || tree == nil {
		return
	}
	current := tree
	for _, p := range path[:len(path)-1] {
		next, ok := current[p].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[p] = next
		}
		current = next
	}
	current[path[len(path)-1]] = value
}

// SaveTree serialises a nested map to YAML.
func SaveTree(tree map[string]any) ([]byte, error) {
	if tree == nil {
		tree = make(map[string]any)
	}
	return yaml.Marshal(tree)
}
