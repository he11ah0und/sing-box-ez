package core

import (
	"regexp"
	"strings"
	"sync"
)

// ansiEscapeRe matches ANSI escape sequences (e.g. \x1b[36m or \x1b[38;5;192m).
var ansiEscapeRe = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")

// CoreLogBuffer stores raw core log lines with an optional line limit.
// Lines are kept exactly as emitted by the core process (including ANSI color
// escape sequences) so the GUI can render them faithfully. Use GetCleanLines
// or StripANSIEscapes when plain text is needed.
type CoreLogBuffer struct {
	mu    sync.Mutex
	limit int
	lines []string
}

// maxLines is a hard ceiling on the buffer size. The configured limit is a
// UI-hinted int and a hand-edited config can set it to 0 or an absurd value;
// without a ceiling an info-level core fills memory on long sessions.
const maxLines = 10000

// clampLimit normalizes a configured limit: non-positive or excessive values
// fall back to the ceiling.
func clampLimit(limit int) int {
	if limit <= 0 || limit > maxLines {
		return maxLines
	}
	return limit
}

// NewCoreLogBuffer creates a new buffer with the given line limit.
func NewCoreLogBuffer(limit int) *CoreLogBuffer {
	return &CoreLogBuffer{
		limit: clampLimit(limit),
		lines: make([]string, 0),
	}
}

// SetLimit updates the line limit. Existing lines are truncated immediately
// if the new limit is smaller than the current count.
func (b *CoreLogBuffer) SetLimit(limit int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.limit = clampLimit(limit)
	if len(b.lines) > b.limit {
		b.lines = b.lines[len(b.lines)-b.limit:]
	}
}

// Add appends a non-empty, trimmed line to the buffer, respecting the limit.
func (b *CoreLogBuffer) Add(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lines = append(b.lines, line)
	if b.limit > 0 && len(b.lines) > b.limit {
		b.lines = b.lines[len(b.lines)-b.limit:]
	}
}

// GetLines returns a copy of the stored lines (raw, may contain ANSI escapes).
func (b *CoreLogBuffer) GetLines() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := make([]string, len(b.lines))
	copy(result, b.lines)
	return result
}

// GetCleanLines returns a copy of the stored lines with ANSI escape sequences
// removed. Useful for clipboard copy or plain-text export.
func (b *CoreLogBuffer) GetCleanLines() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := make([]string, len(b.lines))
	for i, line := range b.lines {
		result[i] = ansiEscapeRe.ReplaceAllString(line, "")
	}
	return result
}

// StripANSIEscapes removes ANSI escape sequences from a string.
func StripANSIEscapes(s string) string {
	return ansiEscapeRe.ReplaceAllString(s, "")
}

// Clear removes all stored lines.
func (b *CoreLogBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lines = b.lines[:0]
}
