package core

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"sing-box-ez/internal/config"
)

// LogLevel represents the severity of a log entry.
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	default:
		return "?"
	}
}

// LogPart is a single structured log entry.
type LogPart struct {
	Timestamp  time.Time
	Level      LogLevel
	Source     *LogTerminal
	Message    string
	SameSource bool
}

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
		Timestamp: time.Now(),
		Level:     level,
		Source:    t,
		Message:   msg,
	})
}

// Logger provides centralized logging with buffering, limits, and core log processing.
type Logger struct {
	cfg     *config.AppConfig
	mu      sync.Mutex
	lines   []string
	parts   []LogPart
	writer  *CoreLogWriter
	manager *Manager

	lastAutoRestart time.Time
	autoRestartMu   sync.Mutex

	// OnAutoRestart is an optional callback invoked when a fatal core error triggers auto-restart.
	OnAutoRestart func()
}

// NewLogger creates a new logger.
func NewLogger(cfg *config.AppConfig, writer *CoreLogWriter, manager *Manager) *Logger {
	return &Logger{
		cfg:    cfg,
		writer: writer,
		manager: manager,
		lines:  make([]string, 0),
		parts:  make([]LogPart, 0),
	}
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

	limit := l.cfg.GetLogLimit()
	if limit > 0 && len(l.lines) > limit {
		l.lines = l.lines[len(l.lines)-limit:]
		l.parts = l.parts[len(l.parts)-limit:]
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

// Start begins reading core process logs in a background goroutine.
func (l *Logger) Start() {
	go l.readLoop()
}

// readLoop polls the core log writer channel and batches lines for processing.
func (l *Logger) readLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	batch := make([]string, 0, 128)
	for {
		select {
		case line, ok := <-l.writer.Ch:
			if !ok {
				if len(batch) > 0 {
					l.processCoreLogs(batch)
				}
				return
			}
			batch = append(batch, line)
		case <-ticker.C:
			if len(batch) > 0 {
				l.processCoreLogs(batch)
				batch = batch[:0]
			}
		}
	}
}

// processCoreLogs handles core output: fatal-error auto-restart and watch-core-logs.
func (l *Logger) processCoreLogs(lines []string) {
	watch := l.cfg.GetWatchCoreLogs()
	restart := l.cfg.GetCoreAutoRestart()
	if !watch && !restart {
		return
	}

	core := l.Root().Allocate("core")

	l.mu.Lock()
	defer l.mu.Unlock()

	for _, msg := range lines {
		msg = strings.TrimRight(msg, "\n")
		if msg == "" {
			continue
		}
		if restart && isCoreFatalError(msg) {
			l.autoRestartMu.Lock()
			if time.Since(l.lastAutoRestart) > 30*time.Second {
				l.lastAutoRestart = time.Now()
				l.autoRestartMu.Unlock()
				core.Info("Detected core fatal error, auto-restarting...")
				go func() {
					if err := l.manager.Restart(); err != nil {
						core.Errorf("Auto-restart failed: %v", err)
					}
				}()
				if l.OnAutoRestart != nil {
					l.OnAutoRestart()
				}
			} else {
				l.autoRestartMu.Unlock()
			}
		}
		if watch {
			core.Info(msg)
		}
	}
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

// Close shuts down the underlying log writer.
func (l *Logger) Close() {
	if l.writer != nil {
		l.writer.Close()
	}
}

// isCoreFatalError detects fatal error patterns in core output.
func isCoreFatalError(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "fatal[") ||
		strings.Contains(lower, "panic:") ||
		strings.Contains(lower, "fetch rule-set") ||
		strings.Contains(lower, "initial rule-set:") ||
		strings.Contains(lower, "save rule-set")
}
