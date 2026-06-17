package logger

import "time"

// LogPart is a single structured log entry.
type LogPart struct {
	Timestamp  time.Time
	Level      LogLevel
	Source     *LogTerminal
	Message    string
	SameSource bool
}
