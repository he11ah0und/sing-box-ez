package logger

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
		return "DBG"
	case LogLevelInfo:
		return "INF"
	case LogLevelWarn:
		return "WRN"
	case LogLevelError:
		return "ERR"
	default:
		return "?"
	}
}
