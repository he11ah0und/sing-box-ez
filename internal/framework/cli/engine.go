// Package cli provides a generic command-line routing engine.
package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// CommandFunc is the signature of a CLI command handler.
// The app argument is the application object the engine is bound to.
type CommandFunc[T any] func(app T, args []string) error

// CommandDef describes a registered CLI command.
type CommandDef[T any] struct {
	Desc string
	Fn   CommandFunc[T]
}

// Engine routes CLI commands to their handlers.
type Engine[T any] struct {
	commands   map[string]CommandDef[T]
	beforeExec func(app T) error
}

// New creates a new CLI engine.
func New[T any]() *Engine[T] {
	return &Engine[T]{
		commands: make(map[string]CommandDef[T]),
	}
}

// Register adds a command to the engine.
func (e *Engine[T]) Register(name, desc string, fn CommandFunc[T]) {
	e.commands[name] = CommandDef[T]{Desc: desc, Fn: fn}
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

// PrintHelp writes auto-generated help to w.
func (e *Engine[T]) PrintHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage:\n")
	fmt.Fprintf(w, "  %s [options] <command>\n", e.appName())
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  --data-dir <path>  Override default data directory")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Commands:")

	names := make([]string, 0, len(e.commands))
	for name := range e.commands {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		fmt.Fprintf(w, "  %-9s %s\n", name, e.commands[name].Desc)
	}
}

// Run parses args and executes the matching command against app.
func (e *Engine[T]) Run(args []string, app T) error {
	if len(args) < 1 {
		e.PrintHelp(os.Stderr)
		return fmt.Errorf("no command specified")
	}

	cmd, ok := e.commands[args[0]]
	if !ok {
		e.PrintHelp(os.Stderr)
		return fmt.Errorf("unknown command: %s", args[0])
	}

	if e.beforeExec != nil {
		if err := e.beforeExec(app); err != nil {
			return err
		}
	}

	return cmd.Fn(app, args[1:])
}
