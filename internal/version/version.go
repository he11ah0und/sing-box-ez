package version

import (
	"fmt"
	"time"

	"sing-box-ez/internal/githuburl"
)

var (
	Branch    = "dev"
	BuildDate = "unknown"
	Commit    = "unknown"
	RepoURL   = githuburl.DefaultProject().RepoURL()

	BuildOS       = "unknown"
	BuildArch     = "unknown"
	BuildGUI      = "unknown"
	BuildBackend  = ""
	BuildCompiler = "unknown"
)

func Info() string {
	switch {
	case BuildDate == "unknown" || BuildDate == "":
		if Commit == "unknown" || Commit == "" {
			return Branch
		}
		return fmt.Sprintf("%s (%s)", Branch, Commit)
	case Commit == "unknown" || Commit == "":
		return fmt.Sprintf("%s (%s)", Branch, BuildDate)
	default:
		return fmt.Sprintf("%s (%s %s)", Branch, BuildDate, Commit)
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

// BuildDateTime parses BuildDate into a time.Time value.
// BuildDate is injected at build time via ldflags in the format "2006-01-02 15:04:05".
func BuildDateTime() (time.Time, error) {
	if BuildDate == "unknown" || BuildDate == "" {
		return time.Time{}, fmt.Errorf("build date unknown")
	}
	return time.Parse(time.DateTime, BuildDate)
}
