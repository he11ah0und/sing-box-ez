// Package config provides a cell-based configuration sheet.
//
// A Sheet is a tree of typed Cells. Each Cell knows its name, type, current
// value and default value. Applications register the schema up front and then
// read or update individual cells by path segments.
package config

import (
	"fmt"
	"reflect"

	"sing-box-ez/internal/framework/logger"
	yamlutil "sing-box-ez/internal/framework/util/yaml"
)

// Type describes the runtime type of a Cell.
type Type string

const (
	TypeBool   Type = "bool"
	TypeInt    Type = "int"
	TypeString Type = "string"
	TypeAny    Type = "any"
)

// Cell is a single typed configuration value.
type Cell struct {
	node *node
}

// Retrieve returns the raw cell value.
func (c *Cell) Retrieve() any {
	if c == nil || c.node == nil {
		return nil
	}
	return c.node.value
}

// Update sets the cell value after validating its type.
func (c *Cell) Update(v any) error {
	if c == nil || c.node == nil {
		return fmt.Errorf("cannot update nil cell")
	}
	if err := validateType(c.node.typ, v); err != nil {
		return fmt.Errorf("cell %s: %w", c.node.pathString(), err)
	}
	c.node.value = v
	return nil
}

// Bool returns the cell value as bool.
func (c *Cell) Bool() bool {
	if c == nil || c.node == nil {
		return false
	}
	v, ok := toBool(c.node.value)
	if ok {
		return v
	}
	v, _ = toBool(c.node.defaultValue)
	return v
}

// Int returns the cell value as int.
func (c *Cell) Int() int {
	if c == nil || c.node == nil {
		return 0
	}
	v, ok := toInt(c.node.value)
	if ok {
		return v
	}
	v, _ = toInt(c.node.defaultValue)
	return v
}

// String returns the cell value as string.
func (c *Cell) String() string {
	if c == nil || c.node == nil {
		return ""
	}
	v, ok := toString(c.node.value)
	if ok {
		return v
	}
	v, _ = toString(c.node.defaultValue)
	return v
}

// Any returns the cell value as-is.
func (c *Cell) Any() any {
	if c == nil || c.node == nil {
		return nil
	}
	if c.node.value != nil {
		return c.node.value
	}
	return c.node.defaultValue
}

// OnMissing controls the behaviour of Sheet.Get for unknown paths.
type OnMissing int

const (
	// OnMissingCreate creates an untyped cell, logs a warning and returns it.
	OnMissingCreate OnMissing = iota
	// OnMissingError returns an error for unknown paths.
	OnMissingError
)

// SheetOptions configures a new Sheet.
type SheetOptions struct {
	// Logger is used to report schema warnings. If nil, warnings are silent.
	Logger *logger.LogTerminal
	// OnMissing is the default behaviour for unknown paths.
	OnMissing OnMissing
}

// Sheet is a tree of typed configuration cells.
type Sheet struct {
	root      *node
	log       *logger.LogTerminal
	onMissing OnMissing
}

type node struct {
	name         string
	typ          Type
	value        any
	defaultValue any
	children     map[string]*node
}

func (n *node) pathString() string {
	if n == nil {
		return ""
	}
	return n.name
}

// NewSheet creates a new empty configuration sheet.
func NewSheet(opts SheetOptions) *Sheet {
	return &Sheet{
		root: &node{
			children: make(map[string]*node),
		},
		log:       opts.Logger,
		onMissing: opts.OnMissing,
	}
}

// Register adds a typed cell to the schema at the given path.
func (s *Sheet) Register(path []string, typ Type, defaultValue any) *Cell {
	if len(path) == 0 {
		return nil
	}
	if err := validateType(typ, defaultValue); err != nil && s.log != nil {
		s.log.Warnf("config schema: invalid default for %v: %v", path, err)
	}
	n := s.ensureNode(path)
	n.typ = typ
	n.defaultValue = defaultValue
	if n.value == nil {
		n.value = defaultValue
	}
	return &Cell{node: n}
}

// Get returns the cell at the given path.
func (s *Sheet) Get(path ...string) (*Cell, error) {
	if len(path) == 0 {
		return nil, fmt.Errorf("empty path")
	}
	n := s.findNode(path)
	if n != nil {
		return &Cell{node: n}, nil
	}

	switch s.onMissing {
	case OnMissingError:
		return nil, fmt.Errorf("config path %v not found", path)
	default:
		if s.log != nil {
			s.log.Warnf("config path %v not defined in schema, creating untyped cell", path)
		}
		return &Cell{node: s.ensureNode(path)}, nil
	}
}

// MustGet returns the cell at the given path. It panics if the path is not
// registered and OnMissing is set to OnMissingError.
func (s *Sheet) MustGet(path ...string) *Cell {
	cell, err := s.Get(path...)
	if err != nil {
		panic(err)
	}
	return cell
}

// Bool is a convenience helper returning the bool value at path.
// Missing cells are created with a warning.
func (s *Sheet) Bool(path ...string) bool {
	return s.warnGet(path).Bool()
}

// Int is a convenience helper returning the int value at path.
// Missing cells are created with a warning.
func (s *Sheet) Int(path ...string) int {
	return s.warnGet(path).Int()
}

// String is a convenience helper returning the string value at path.
// Missing cells are created with a warning.
func (s *Sheet) String(path ...string) string {
	return s.warnGet(path).String()
}

// Set is a convenience helper updating the value at path.
// Missing cells are created with a warning.
func (s *Sheet) Set(path []string, value any) error {
	return s.warnGet(path).Update(value)
}

func (s *Sheet) warnGet(path []string) *Cell {
	if len(path) == 0 {
		return &Cell{}
	}
	n := s.findNode(path)
	if n == nil {
		if s.log != nil {
			s.log.Warnf("config path %v not defined in schema, creating untyped cell", path)
		}
		n = s.ensureNode(path)
	}
	return &Cell{node: n}
}

// LoadYAML parses YAML data and updates registered cells. Unknown keys log
// warnings.
func (s *Sheet) LoadYAML(data []byte) error {
	raw, err := yamlutil.LoadTree(data)
	if err != nil {
		return err
	}
	s.loadTree(raw, nil)
	return nil
}

func (s *Sheet) loadTree(tree map[string]any, path []string) {
	for key, value := range tree {
		currentPath := append(path, key)
		switch v := value.(type) {
		case map[string]any:
			s.loadTree(v, currentPath)
		default:
			n := s.findNode(currentPath)
			if n == nil {
				if s.log != nil {
					s.log.Warnf("config path %v not defined in schema", currentPath)
				}
				n = s.ensureNode(currentPath)
			}
			normalized, err := normalizeValue(n.typ, v)
			if err != nil {
				if s.log != nil {
					s.log.Warnf("config path %v: %v", currentPath, err)
				}
				continue
			}
			if err := n.applyValue(normalized); err != nil {
				if s.log != nil {
					s.log.Warnf("config path %v: %v", currentPath, err)
				}
			}
		}
	}
}

// SaveYAML serialises all registered cells to YAML.
func (s *Sheet) SaveYAML() ([]byte, error) {
	tree := make(map[string]any)
	s.saveNode(s.root, tree)
	return yamlutil.SaveTree(tree)
}

func (s *Sheet) saveNode(n *node, tree map[string]any) {
	if n == nil {
		return
	}
	for name, child := range n.children {
		if len(child.children) == 0 {
			if child.value != nil {
				tree[name] = child.value
			} else if child.defaultValue != nil {
				tree[name] = child.defaultValue
			}
		} else {
			sub := make(map[string]any)
			s.saveNode(child, sub)
			if len(sub) > 0 {
				tree[name] = sub
			}
		}
	}
}

// Config is the minimal interface required by framework services.
type Config interface {
	Get(path ...string) (*Cell, error)
	MustGet(path ...string) *Cell
}

func (n *node) applyValue(v any) error {
	if err := validateType(n.typ, v); err != nil {
		return err
	}
	n.value = v
	return nil
}

func (s *Sheet) findNode(path []string) *node {
	current := s.root
	for _, p := range path {
		if current == nil {
			return nil
		}
		next, ok := current.children[p]
		if !ok {
			return nil
		}
		current = next
	}
	return current
}

func (s *Sheet) ensureNode(path []string) *node {
	current := s.root
	for _, p := range path {
		if current.children == nil {
			current.children = make(map[string]*node)
		}
		next, ok := current.children[p]
		if !ok {
			next = &node{
				name:     p,
				children: make(map[string]*node),
			}
			current.children[p] = next
		}
		current = next
	}
	return current
}

func validateType(typ Type, v any) error {
	if v == nil {
		return nil
	}
	switch typ {
	case TypeBool:
		if _, ok := toBool(v); !ok {
			return fmt.Errorf("expected bool, got %T", v)
		}
	case TypeInt:
		if _, ok := toInt(v); !ok {
			return fmt.Errorf("expected int, got %T", v)
		}
	case TypeString:
		if _, ok := toString(v); !ok {
			return fmt.Errorf("expected string, got %T", v)
		}
	case TypeAny, "":
		return nil
	default:
		return fmt.Errorf("unknown type %q", typ)
	}
	return nil
}

func normalizeValue(typ Type, v any) (any, error) {
	switch typ {
	case TypeBool:
		b, ok := toBool(v)
		if !ok {
			return nil, fmt.Errorf("expected bool, got %T", v)
		}
		return b, nil
	case TypeInt:
		i, ok := toInt(v)
		if !ok {
			return nil, fmt.Errorf("expected int, got %T", v)
		}
		return i, nil
	case TypeString:
		str, ok := toString(v)
		if !ok {
			return nil, fmt.Errorf("expected string, got %T", v)
		}
		return str, nil
	case TypeAny, "":
		return v, nil
	default:
		return nil, fmt.Errorf("unknown type %q", typ)
	}
}

func toBool(v any) (bool, bool) {
	switch x := v.(type) {
	case bool:
		return x, true
	case string:
		return x == "true" || x == "1" || x == "yes", true
	default:
		return false, false
	}
}

func toInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int8:
		return int(x), true
	case int16:
		return int(x), true
	case int32:
		return int(x), true
	case int64:
		return int(x), true
	case uint:
		return int(x), true
	case uint8:
		return int(x), true
	case uint16:
		return int(x), true
	case uint32:
		return int(x), true
	case uint64:
		return int(x), true
	case float32:
		return int(x), true
	case float64:
		return int(x), true
	default:
		return 0, false
	}
}

func toString(v any) (string, bool) {
	if v == nil {
		return "", true
	}
	if s, ok := v.(string); ok {
		return s, true
	}
	return "", false
}

// TypeOf returns the Type constant matching the given Go value.
func TypeOf(v any) Type {
	if v == nil {
		return TypeAny
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Bool:
		return TypeBool
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return TypeInt
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return TypeInt
	case reflect.Float32, reflect.Float64:
		return TypeInt
	case reflect.String:
		return TypeString
	default:
		return TypeAny
	}
}
