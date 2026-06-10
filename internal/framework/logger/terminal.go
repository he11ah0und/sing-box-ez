package logger

import "fmt"

// LogTerminal represents a node in a hierarchical logging tree.
// The root terminal has id "root" and nil parent; child terminals
// are created with Allocate and form paths such as root/core/start.
type LogTerminal struct {
	logger *Logger
	id     string
	parent *LogTerminal
}

// FullPath returns the slash-separated path of this terminal
// (e.g. "root/core/start").
func (t *LogTerminal) FullPath() string {
	if t.parent == nil {
		return t.id
	}
	return t.parent.FullPath() + "/" + t.id
}

// BlockPath returns the bracketed path used in log output
// (e.g. "[root][core][start]").
func (t *LogTerminal) BlockPath() string {
	if t.parent == nil {
		return "[" + t.id + "]"
	}
	return t.parent.BlockPath() + "[" + t.id + "]"
}

// Allocate creates a child terminal with the given id.
func (t *LogTerminal) Allocate(id string) *LogTerminal {
	return &LogTerminal{logger: t.logger, id: id, parent: t}
}

// Debug logs a debug-level message from this terminal.
func (t *LogTerminal) Debug(msg string) { t.log(LogLevelDebug, msg) }

// Info logs an info-level message from this terminal.
func (t *LogTerminal) Info(msg string) { t.log(LogLevelInfo, msg) }

// Warn logs a warning-level message from this terminal.
func (t *LogTerminal) Warn(msg string) { t.log(LogLevelWarn, msg) }

// Error logs an error-level message from this terminal.
func (t *LogTerminal) Error(msg string) { t.log(LogLevelError, msg) }

// Errorf logs a formatted error-level message from this terminal.
func (t *LogTerminal) Errorf(format string, args ...interface{}) {
	t.Error(fmt.Sprintf(format, args...))
}

// Infof logs a formatted info-level message from this terminal.
func (t *LogTerminal) Infof(format string, args ...interface{}) {
	t.Info(fmt.Sprintf(format, args...))
}

func (t *LogTerminal) log(level LogLevel, msg string) {
	t.logger.append(&LogPart{
		Level:   level,
		Source:  t,
		Message: msg,
	})
}
