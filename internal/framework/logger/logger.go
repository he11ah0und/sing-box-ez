package logger

import (
	"fmt"
	"sync"
	"time"
)

// Logger provides centralized logging with buffering and an optional line limit.
type Logger struct {
	mu    sync.Mutex
	limit int
	lines []string
	parts []LogPart
}

// NewLogger creates a new logger with the given line limit (0 = unlimited).
func NewLogger(limit int) *Logger {
	return &Logger{
		limit: limit,
		lines: make([]string, 0),
		parts: make([]LogPart, 0),
	}
}

// SetLimit updates the stored line limit. Existing lines are not truncated
// until the next append.
func (l *Logger) SetLimit(limit int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.limit = limit
}

// Root returns the root logging terminal. All scoped loggers should be
// allocated from this root so that log output contains the full block path.
func (l *Logger) Root() *LogTerminal {
	return &LogTerminal{logger: l, id: "root"}
}

// Log appends a user-facing message at info level from the root terminal.
// Kept for backwards compatibility.
func (l *Logger) Log(msg string) {
	l.Root().Info(msg)
}

// append adds a structured log part and renders it to a text line.
func (l *Logger) append(part *LogPart) {
	part.Timestamp = time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.parts) > 0 {
		last := l.parts[len(l.parts)-1]
		if last.Source.FullPath() == part.Source.FullPath() {
			part.SameSource = true
		}
	}

	l.parts = append(l.parts, *part)
	line := l.renderLocked(part)
	l.lines = append(l.lines, line)

	if l.limit > 0 && len(l.lines) > l.limit {
		l.lines = l.lines[len(l.lines)-l.limit:]
		l.parts = l.parts[len(l.parts)-l.limit:]
	}
}

// renderLocked formats a LogPart into a text line.
func (l *Logger) renderLocked(part *LogPart) string {
	timestamp := part.Timestamp.Format("15:04:05")
	var source string
	if part.SameSource {
		source = "[" + part.Source.id + "]"
	} else {
		source = part.Source.BlockPath()
	}
	return fmt.Sprintf("[%s] [%s] %s -> %s", timestamp, part.Level.String(), source, part.Message)
}

// GetLines returns a copy of the current log lines.
func (l *Logger) GetLines() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]string, len(l.lines))
	copy(result, l.lines)
	return result
}

// Clear removes all stored log lines.
func (l *Logger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = l.lines[:0]
	l.parts = l.parts[:0]
}
