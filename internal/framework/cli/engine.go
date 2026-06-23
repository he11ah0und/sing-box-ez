// Package cli provides a generic command-line routing engine with typed
// positional arguments and flags.
package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CommandFunc is the signature of a CLI command handler.
// The parsed arguments and flags are available through ctx.
type CommandFunc[T any] func(app T, ctx *Context) error

// CommandDef describes a registered CLI command.
type CommandDef[T any] struct {
	Desc  string
	Args  []Arg
	Flags []Flag
	Fn    CommandFunc[T]
}

// CommandBuilder allows chaining command flag declarations.
type CommandBuilder[T any] struct {
	engine *Engine[T]
	cmd    *CommandDef[T]
}

// Flag adds a command-specific flag and returns the builder.
func (b *CommandBuilder[T]) Flag(f Flag) *CommandBuilder[T] {
	b.cmd.Flags = append(b.cmd.Flags, f)
	return b
}

// Engine routes CLI commands and parses their arguments.
type Engine[T any] struct {
	commands     map[string]*CommandDef[T]
	globalFlags  []Flag
	globalValues map[string]Value
	beforeExec   func(app T) error
}

// New creates a new CLI engine with the built-in --test global flag.
func New[T any]() *Engine[T] {
	e := &Engine[T]{
		commands:     make(map[string]*CommandDef[T]),
		globalFlags:  make([]Flag, 0),
		globalValues: make(map[string]Value),
	}
	e.AddGlobalFlag(Flag{
		Name:  "help",
		Short: 'h',
		Desc:  "Show this help message",
		Type:  Bool,
	})
	e.AddGlobalFlag(Flag{
		Name: "test",
		Desc: "Validate arguments without executing the command",
		Type: Bool,
	})
	return e
}

// Register adds a command to the engine.
func (e *Engine[T]) Register(name, desc string, fn CommandFunc[T], args ...Arg) *CommandBuilder[T] {
	cmd := &CommandDef[T]{
		Desc: desc,
		Args: args,
		Fn:   fn,
	}
	e.commands[name] = cmd
	return &CommandBuilder[T]{engine: e, cmd: cmd}
}

// AddGlobalFlag registers a flag that must appear before the command name.
func (e *Engine[T]) AddGlobalFlag(f Flag) {
	e.globalFlags = append(e.globalFlags, f)
}

// SetBeforeExec registers a hook invoked before each command runs.
func (e *Engine[T]) SetBeforeExec(fn func(app T) error) {
	e.beforeExec = fn
}

// appName returns the executable name as seen by the user.
func (e *Engine[T]) appName() string {
	if len(os.Args) == 0 {
		return ""
	}
	return filepath.Base(os.Args[0])
}

// ParseGlobals parses global flags that appear before the command name.
// It returns the parsed global values and the remaining arguments (command + its args).
func (e *Engine[T]) ParseGlobals(args []string) (map[string]Value, []string, error) {
	values := make(map[string]Value)
	i := 0
	for i < len(args) {
		tok := args[i]
		if tok == "--" {
			i++
			break
		}
		if !isFlag(tok) {
			break
		}

		name, short, value, hasValue, err := splitFlagToken(tok)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid flag %q: %w", tok, err)
		}

		def := e.findGlobalFlag(name, short)
		if def == nil {
			return nil, nil, fmt.Errorf("unknown global flag %q", tok)
		}

		var raw string
		if hasValue {
			raw = value
			i++
		} else if isBoolType(def.Type) {
			raw = "true"
			i++
		} else {
			i++
			if i >= len(args) {
				return nil, nil, fmt.Errorf("flag --%s requires a value", def.Name)
			}
			raw = args[i]
			i++
		}

		v, err := def.Type.Parse(raw)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid value for --%s: %w", def.Name, err)
		}
		values[def.Name] = v
	}

	e.globalValues = values
	return values, args[i:], nil
}

// Run parses args and executes the matching command against app.
// Global flags must already have been parsed with ParseGlobals.
func (e *Engine[T]) Run(args []string, app T) error {
	if e.helpRequested() {
		e.PrintHelp(os.Stdout)
		return nil
	}

	if len(args) < 1 {
		e.PrintHelp(os.Stderr)
		return fmt.Errorf("no command specified")
	}

	cmdName := args[0]
	if cmdName == "help" {
		if len(args) > 1 {
			e.printCommandHelp(os.Stderr, args[1])
		} else {
			e.PrintHelp(os.Stderr)
		}
		return nil
	}

	cmd, ok := e.commands[cmdName]
	if !ok {
		e.PrintHelp(os.Stderr)
		return fmt.Errorf("unknown command: %s", cmdName)
	}

	ctx, err := e.parseCommand(cmdName, cmd, args[1:])
	if err != nil {
		e.printCommandUsage(os.Stderr, cmdName, cmd)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}

	if e.testMode() {
		e.printTestReport(os.Stderr, cmdName, cmd, ctx)
		return nil
	}

	if e.beforeExec != nil {
		if err := e.beforeExec(app); err != nil {
			return err
		}
	}

	return cmd.Fn(app, ctx)
}

// PrintHelp writes auto-generated help to w.
func (e *Engine[T]) PrintHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage:\n")
	fmt.Fprintf(w, "  %s [global options] <command> [command options]\n", e.appName())
	fmt.Fprintln(w, "")

	if len(e.globalFlags) > 0 {
		fmt.Fprintln(w, "Global options:")
		for _, f := range e.globalFlags {
			fmt.Fprintf(w, "  %s\n", formatFlagHelp(f))
		}
		fmt.Fprintln(w, "")
	}

	fmt.Fprintln(w, "Commands:")
	names := make([]string, 0, len(e.commands))
	for name := range e.commands {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		cmd := e.commands[name]
		fmt.Fprintf(w, "  %-12s %s\n", name, cmd.Desc)
		usage := commandUsage(name, cmd)
		if usage != name {
			fmt.Fprintf(w, "                %s\n", usage)
		}
	}
}

func (e *Engine[T]) testMode() bool {
	if v, ok := e.globalValues["test"]; ok {
		return AsBool(v)
	}
	return false
}

func (e *Engine[T]) helpRequested() bool {
	if v, ok := e.globalValues["help"]; ok {
		return AsBool(v)
	}
	return false
}

// HelpRequested reports whether the user asked for help via a global flag.
func (e *Engine[T]) HelpRequested() bool {
	return e.helpRequested()
}

func (e *Engine[T]) findGlobalFlag(name string, short rune) *Flag {
	for i := range e.globalFlags {
		f := &e.globalFlags[i]
		if name != "" && f.Name == name {
			return f
		}
		if short != 0 && f.Short == short {
			return f
		}
	}
	return nil
}

func (e *Engine[T]) findCommandFlag(cmd *CommandDef[T], name string, short rune) *Flag {
	for i := range cmd.Flags {
		f := &cmd.Flags[i]
		if name != "" && f.Name == name {
			return f
		}
		if short != 0 && f.Short == short {
			return f
		}
	}
	return nil
}

type parsedItem struct {
	name  string
	desc  string
	raw   string
	value Value
	err   error
}

func (e *Engine[T]) parseCommand(cmdName string, cmd *CommandDef[T], tokens []string) (*Context, error) {
	positionals := make([]string, 0, len(tokens))
	flags := make(map[string]Value)
	flagItems := make([]parsedItem, 0, len(cmd.Flags))
	var errs []string

	i := 0
	for i < len(tokens) {
		tok := tokens[i]
		if tok == "--" {
			i++
			for i < len(tokens) {
				positionals = append(positionals, tokens[i])
				i++
			}
			break
		}
		if !isFlag(tok) {
			positionals = append(positionals, tok)
			i++
			continue
		}

		name, short, inlineValue, hasInline, err := splitFlagToken(tok)
		if err != nil {
			errs = append(errs, fmt.Sprintf("invalid flag %q", tok))
			i++
			continue
		}

		def := e.findCommandFlag(cmd, name, short)
		if def == nil {
			errs = append(errs, fmt.Sprintf("unknown flag %q", tok))
			i++
			continue
		}

		var raw string
		if hasInline {
			raw = inlineValue
			i++
		} else if isBoolType(def.Type) {
			raw = "true"
			i++
		} else {
			i++
			if i >= len(tokens) {
				errs = append(errs, fmt.Sprintf("flag --%s requires a value", def.Name))
				break
			}
			raw = tokens[i]
			i++
		}

		v, err := def.Type.Parse(raw)
		flagItems = append(flagItems, parsedItem{name: def.Name, desc: def.Desc, raw: raw, value: v, err: err})
		if err == nil {
			flags[def.Name] = v
		} else {
			errs = append(errs, fmt.Sprintf("invalid value for flag --%s: %v", def.Name, err))
		}
	}

	// Apply flag defaults.
	for _, f := range cmd.Flags {
		if _, ok := flags[f.Name]; !ok && f.Default != nil {
			flags[f.Name] = f.Default
		}
	}

	// Validate and parse positional arguments.
	required := 0
	optional := 0
	for _, a := range cmd.Args {
		if a.Optional {
			optional++
		} else {
			required++
		}
	}

	argItems := make([]parsedItem, 0, len(cmd.Args))
	argValues := make(map[string]Value)

	if len(positionals) < required {
		for idx := len(positionals); idx < required; idx++ {
			def := cmd.Args[idx]
			argItems = append(argItems, parsedItem{name: def.Name, desc: def.Desc, err: fmt.Errorf("missing required argument")})
		}
		errs = append(errs, "missing required arguments")
	} else if len(positionals) > required+optional {
		for idx := required + optional; idx < len(positionals); idx++ {
			errs = append(errs, fmt.Sprintf("unexpected argument %q", positionals[idx]))
		}
	}

	for idx, def := range cmd.Args {
		var raw string
		var has bool
		if idx < len(positionals) {
			raw = positionals[idx]
			has = true
		} else if def.Optional && def.Default != nil {
			argValues[def.Name] = def.Default
			argItems = append(argItems, parsedItem{name: def.Name, desc: def.Desc, raw: def.Default.String(), value: def.Default})
			continue
		}

		if !has {
			continue
		}

		t := def.Type
		if t == nil {
			t = String
		}
		v, err := t.Parse(raw)
		argItems = append(argItems, parsedItem{name: def.Name, desc: def.Desc, raw: raw, value: v, err: err})
		if err == nil {
			argValues[def.Name] = v
		} else {
			errs = append(errs, fmt.Sprintf("invalid value for argument %q: %v", def.Name, err))
		}
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(errs, "; "))
	}

	return &Context{
		command: cmdName,
		args:    argValues,
		flags:   flags,
		globals: e.globalValues,
	}, nil
}

func (e *Engine[T]) printCommandUsage(w io.Writer, name string, cmd *CommandDef[T]) {
	fmt.Fprintf(w, "Usage: %s\n", commandUsage(name, cmd))
	if cmd.Desc != "" {
		fmt.Fprintf(w, "\n%s\n", cmd.Desc)
	}
	if len(cmd.Args) > 0 {
		fmt.Fprintln(w, "\nArguments:")
		for _, a := range cmd.Args {
			name := a.Name
			if a.Optional {
				name = "[" + name + "]"
			} else {
				name = "<" + name + ">"
			}
			fmt.Fprintf(w, "  %-12s %s\n", name, a.Desc)
			if a.Type != nil {
				fmt.Fprintf(w, "               type: %s (%s)\n", a.Type.Name(), a.Type.String())
			}
		}
	}
	if len(cmd.Flags) > 0 {
		fmt.Fprintln(w, "\nOptions:")
		for _, f := range cmd.Flags {
			fmt.Fprintf(w, "  %s\n", formatFlagHelp(f))
		}
	}
}

func (e *Engine[T]) printCommandHelp(w io.Writer, name string) {
	cmd, ok := e.commands[name]
	if !ok {
		fmt.Fprintf(w, "Unknown command: %s\n", name)
		e.PrintHelp(w)
		return
	}
	e.printCommandUsage(w, name, cmd)
}

func (e *Engine[T]) printTestReport(w io.Writer, name string, cmd *CommandDef[T], ctx *Context) {
	fmt.Fprintf(w, "Validation report for command %q\n", name)
	fmt.Fprintln(w)
	if len(cmd.Args) > 0 {
		fmt.Fprintln(w, "Arguments:")
		for _, a := range cmd.Args {
			v := ctx.Arg(a.Name)
			status := "valid"
			if v == nil {
				status = "missing"
			}
			fmt.Fprintf(w, "  %-12s %q [%s]\n", a.Name, v, status)
		}
	}
	if len(cmd.Flags) > 0 {
		fmt.Fprintln(w, "Options:")
		for _, f := range cmd.Flags {
			v := ctx.Flag(f.Name)
			status := "set"
			if v == nil {
				status = "not set"
				if f.Default != nil {
					v = f.Default
					status = "default"
				}
			}
			fmt.Fprintf(w, "  --%-10s %q [%s]\n", f.Name, v, status)
		}
	}
}

func commandUsage[T any](name string, cmd *CommandDef[T]) string {
	var parts []string
	parts = append(parts, name)
	for _, a := range cmd.Args {
		if a.Optional {
			parts = append(parts, "["+a.Name+"]")
		} else {
			parts = append(parts, "<"+a.Name+">")
		}
	}
	if len(cmd.Flags) > 0 {
		parts = append(parts, "[options]")
	}
	return strings.Join(parts, " ")
}

func formatFlagHelp(f Flag) string {
	var names []string
	if f.Short != 0 {
		names = append(names, fmt.Sprintf("-%c", f.Short))
	}
	names = append(names, fmt.Sprintf("--%s", f.Name))

	meta := ""
	if !isBoolType(f.Type) {
		if f.Type != nil {
			meta = fmt.Sprintf(" <%s>", f.Type.Name())
		} else {
			meta = " <value>"
		}
	}

	return fmt.Sprintf("%-24s %s", strings.Join(names, ", ")+meta, f.Desc)
}

func isBoolType(t Type) bool {
	if t == nil {
		return false
	}
	return t.Name() == Bool.Name()
}
