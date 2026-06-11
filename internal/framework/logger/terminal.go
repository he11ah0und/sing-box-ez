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

// Debugf logs a debug-level message from this terminal.
// When the first argument is a format string and additional arguments are
// provided, fmt.Sprintf is used; otherwise the arguments are printed with
// fmt.Sprint. This avoids go vet format-string warnings while keeping a
// single convenient API.
func (t *LogTerminal) Debugf(v ...interface{}) {
	t.writef(LogLevelDebug, v)
}

// Infof logs an info-level message from this terminal.
func (t *LogTerminal) Infof(v ...interface{}) {
	t.writef(LogLevelInfo, v)
}

// Warnf logs a warning-level message from this terminal.
func (t *LogTerminal) Warnf(v ...interface{}) {
	t.writef(LogLevelWarn, v)
}

// Errorf logs an error-level message from this terminal.
func (t *LogTerminal) Errorf(v ...interface{}) {
	t.writef(LogLevelError, v)
}

func (t *LogTerminal) writef(level LogLevel, v []interface{}) {
	if t == nil || t.logger == nil {
		return
	}
	var msg string
	if len(v) > 0 {
		if format, ok := v[0].(string); ok && len(v) > 1 {
			msg = fmt.Sprintf(format, v[1:]...)
		} else {
			msg = fmt.Sprint(v...)
		}
	}
	t.log(level, msg)
}

func (t *LogTerminal) log(level LogLevel, msg string) {
	if t == nil || t.logger == nil {
		return
	}
	t.logger.append(&LogPart{
		Level:   level,
		Source:  t,
		Message: msg,
	})
}
