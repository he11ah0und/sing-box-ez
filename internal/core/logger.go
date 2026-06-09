package core

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"sing-box-ez/internal/config"
)

// Logger provides centralized logging with buffering, limits, and core log processing.
type Logger struct {
	cfg     *config.AppConfig
	mu      sync.Mutex
	lines   []string
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
		cfg:     cfg,
		writer:  writer,
		manager: manager,
		lines:   make([]string, 0),
	}
}

// Start begins reading core process logs in a background goroutine.
func (l *Logger) Start() {
	go l.readLoop()
}

// Log appends a user-facing message with a timestamp.
func (l *Logger) Log(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.appendLocked(msg)
}

// appendLocked adds a line while holding the lock.
func (l *Logger) appendLocked(msg string) {
	timestamp := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] %s", timestamp, msg)
	l.lines = append(l.lines, line)
	limit := l.cfg.GetLogLimit()
	if limit > 0 && len(l.lines) > limit {
		l.lines = l.lines[len(l.lines)-limit:]
	}
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
				l.appendLocked("Detected core fatal error, auto-restarting...")
				go func() {
					if err := l.manager.Restart(); err != nil {
						l.Log("Auto-restart failed: " + err.Error())
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
			l.appendLocked("[core] " + msg)
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
