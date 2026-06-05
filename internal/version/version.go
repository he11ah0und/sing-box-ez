package version

import (
	"fmt"
	"time"
)

var (
	Branch    = "dev"
	BuildDate = "unknown"
	Commit    = "unknown"

	BuildOS       = "unknown"
	BuildArch     = "unknown"
	BuildGUI      = "unknown"
	BuildBackend  = ""
	BuildCompiler = "unknown"
)

func Info() string {
	bd := buildDateString()
	switch {
	case bd == "":
		if Commit == "unknown" || Commit == "" {
			return Branch
		}
		return fmt.Sprintf("%s (%s)", Branch, Commit)
	case Commit == "unknown" || Commit == "":
		return fmt.Sprintf("%s (%s)", Branch, bd)
	default:
		return fmt.Sprintf("%s (%s %s)", Branch, bd, Commit)
	}
}

func BuildFlags() string {
	s := BuildArch + "-" + BuildOS
	if BuildCompiler != "" && BuildCompiler != "unknown" {
		s += "-" + BuildCompiler
	}
	if BuildGUI == "1" {
		s += "-gui"
	} else if BuildGUI == "0" {
		s += "-cli"
	}
	if BuildBackend != "" {
		s += "-" + BuildBackend
	}
	return s
}

// BuildDateTime parses BuildDate (UTC RFC3339) into a time.Time in the local timezone.
func BuildDateTime() (time.Time, error) {
	if BuildDate == "unknown" || BuildDate == "" {
		return time.Time{}, fmt.Errorf("build date unknown")
	}
	t, err := time.Parse(time.RFC3339, BuildDate)
	if err != nil {
		return time.Time{}, err
	}
	return t.Local(), nil
}

// buildDateString returns a human-readable local date string ("2006-01-02 15:04:05"),
// or the raw BuildDate value if parsing fails.
func buildDateString() string {
	if BuildDate == "unknown" || BuildDate == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, BuildDate)
	if err != nil {
		return BuildDate
	}
	return t.Local().Format("2006-01-02 15:04:05")
}
