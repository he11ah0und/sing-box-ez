package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Value is a parsed CLI argument or flag value.
type Value interface {
	fmt.Stringer
}

// Type parses raw string input into a Value.
// Developers can implement their own types and register them via Arg/Flag.
type Type interface {
	Name() string
	Parse(raw string) (Value, error)
	// String returns a human-readable description of accepted values.
	String() string
}

// Arg describes a positional command argument.
type Arg struct {
	Name     string
	Desc     string
	Type     Type
	Optional bool
	Default  Value
}

// Flag describes a command-line flag.
type Flag struct {
	Name    string
	Short   rune
	Desc    string
	Type    Type
	Default Value
}

// Context holds parsed arguments, flags and global flags for a command invocation.
type Context struct {
	command string
	args    map[string]Value
	flags   map[string]Value
	globals map[string]Value
}

// Arg returns the parsed positional argument by name.
func (c *Context) Arg(name string) Value {
	return c.args[name]
}

// Flag returns the parsed command flag by name.
func (c *Context) Flag(name string) Value {
	return c.flags[name]
}

// Global returns a parsed global flag by name.
func (c *Context) Global(name string) Value {
	return c.globals[name]
}

// Command returns the name of the command being executed.
func (c *Context) Command() string {
	return c.command
}

// Args returns a copy of all parsed positional arguments.
func (c *Context) Args() map[string]Value {
	out := make(map[string]Value, len(c.args))
	for k, v := range c.args {
		out[k] = v
	}
	return out
}

// Built-in value types.

type StringValue string

func (v StringValue) String() string { return string(v) }

type IntValue int64

func (v IntValue) String() string { return strconv.FormatInt(int64(v), 10) }

type FloatValue float64

func (v FloatValue) String() string { return strconv.FormatFloat(float64(v), 'f', -1, 64) }

type BoolValue bool

func (v BoolValue) String() string { return strconv.FormatBool(bool(v)) }

// Built-in types.

// String accepts any non-empty string.
var String Type = &stringType{}

type stringType struct{}

func (t *stringType) Name() string { return "string" }

func (t *stringType) Parse(raw string) (Value, error) {
	return StringValue(raw), nil
}

func (t *stringType) String() string { return "any text" }

// Int accepts base-10 integers.
var Int Type = &intType{}

type intType struct{}

func (t *intType) Name() string { return "int" }

func (t *intType) Parse(raw string) (Value, error) {
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("expected integer, got %q", raw)
	}
	return IntValue(v), nil
}

func (t *intType) String() string { return "integer" }

// Float accepts decimal numbers.
var Float Type = &floatType{}

type floatType struct{}

func (t *floatType) Name() string { return "float" }

func (t *floatType) Parse(raw string) (Value, error) {
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, fmt.Errorf("expected number, got %q", raw)
	}
	return FloatValue(v), nil
}

func (t *floatType) String() string { return "number" }

// Bool accepts true/false.
var Bool Type = &boolType{}

type boolType struct{}

func (t *boolType) Name() string { return "bool" }

func (t *boolType) Parse(raw string) (Value, error) {
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, fmt.Errorf("expected true/false, got %q", raw)
	}
	return BoolValue(v), nil
}

func (t *boolType) String() string { return "true|false" }

// Path accepts a non-empty path string.
var Path Type = &pathType{}

type pathType struct{}

func (t *pathType) Name() string { return "path" }

func (t *pathType) Parse(raw string) (Value, error) {
	if raw == "" {
		return nil, fmt.Errorf("path cannot be empty")
	}
	return StringValue(raw), nil
}

func (t *pathType) String() string { return "file path" }

// OSPath returns a Type that validates a path string.
// If mustExist is true it also checks that the path exists.
func OSPath(mustExist bool) Type {
	return &osPathType{mustExist: mustExist}
}

type osPathType struct {
	mustExist bool
}

func (t *osPathType) Name() string { return "os-path" }

func (t *osPathType) Parse(raw string) (Value, error) {
	if raw == "" {
		return nil, fmt.Errorf("path cannot be empty")
	}
	if t.mustExist {
		if _, err := os.Stat(raw); err != nil {
			return nil, fmt.Errorf("path %q is not accessible: %w", raw, err)
		}
	}
	return StringValue(raw), nil
}

func (t *osPathType) String() string {
	if t.mustExist {
		return "existing file or directory path"
	}
	return "file path"
}

// AsString converts a Value to a string.
func AsString(v Value) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(StringValue); ok {
		return string(s)
	}
	return v.String()
}

// AsInt converts a Value to an int64.
func AsInt(v Value) int64 {
	if v == nil {
		return 0
	}
	switch t := v.(type) {
	case IntValue:
		return int64(t)
	case FloatValue:
		return int64(t)
	default:
		if i, err := strconv.ParseInt(v.String(), 10, 64); err == nil {
			return i
		}
		return 0
	}
}

// AsFloat converts a Value to a float64.
func AsFloat(v Value) float64 {
	if v == nil {
		return 0
	}
	switch t := v.(type) {
	case FloatValue:
		return float64(t)
	case IntValue:
		return float64(t)
	default:
		if f, err := strconv.ParseFloat(v.String(), 64); err == nil {
			return f
		}
		return 0
	}
}

// AsBool converts a Value to a bool.
func AsBool(v Value) bool {
	if v == nil {
		return false
	}
	if b, ok := v.(BoolValue); ok {
		return bool(b)
	}
	if b, err := strconv.ParseBool(v.String()); err == nil {
		return b
	}
	return false
}

// normalizeFlagName returns the canonical flag name.
func normalizeFlagName(name string) string {
	return name
}

// isFlag reports whether token looks like a flag.
func isFlag(token string) bool {
	return len(token) >= 2 && token[0] == '-'
}

// splitFlagToken parses a flag token such as --name, --name=value, -n or -n=value.
// It returns the long name, short rune (if any), the optional value, whether a value
// was present inline, and an error if malformed.
func splitFlagToken(token string) (name string, short rune, value string, hasValue bool, err error) {
	if len(token) < 2 || token[0] != '-' {
		return "", 0, "", false, fmt.Errorf("not a flag")
	}

	if token[1] == '-' {
		body := token[2:]
		if body == "" {
			return "", 0, "", false, fmt.Errorf("empty flag name")
		}
		if idx := indexRune(body, '='); idx >= 0 {
			return body[:idx], 0, body[idx+1:], true, nil
		}
		return body, 0, "", false, nil
	}

	// Short flag: -f or -f=value.
	body := token[1:]
	short = rune(body[0])
	if len(body) == 1 {
		return "", short, "", false, nil
	}
	if body[1] == '=' {
		return "", short, body[2:], true, nil
	}
	return "", 0, "", false, fmt.Errorf("short flags must be a single letter")
}

func indexRune(s string, r rune) int {
	for i, c := range s {
		if c == r {
			return i
		}
	}
	return -1
}

// expandPath resolves a path to an absolute path if it is relative.
func expandPath(raw string) string {
	if raw == "" || filepath.IsAbs(raw) {
		return raw
	}
	if abs, err := filepath.Abs(raw); err == nil {
		return abs
	}
	return raw
}
