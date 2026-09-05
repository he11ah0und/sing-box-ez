package version

import (
	"fmt"
	"time"
)

var (
	// Version is the semver of the build, injected from the VERSION file via
	// ldflags (without the leading "v"). Empty for builds made outside the
	// Makefile.
	Version   = ""
	Branch    = "unknown"
	BuildDate = "unknown"
	Commit    = "unknown"

	BuildOS       = "unknown"
	BuildArch     = "unknown"
	BuildGUI      = "unknown"
	BuildBackend  = "unknown"
	BuildCompiler = "unknown"
	BuildDev      = "true"
	CommitDate    = "unknown"
)

// Tag returns the release tag for this build ("v"+Version), or "" when the
// build carries no semver.
func Tag() string {
	if Version == "" || Version == "unknown" {
		return ""
	}
	return "v" + Version
}

func Info() string {
	bd := buildDateString()
	label := Branch
	if Tag() != "" {
		label = Tag()
	}
	switch {
	case bd == "":
		if Commit == "unknown" || Commit == "" {
			return label
		}
		return fmt.Sprintf("%s (%s)", label, Commit)
	case Commit == "unknown" || Commit == "":
		return fmt.Sprintf("%s (%s)", label, bd)
	default:
		return fmt.Sprintf("%s (%s %s)", label, bd, Commit)
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
	if BuildDev == "true" {
		s += "-dev"
	}
	return s
}

// IsDev reports whether this is a development build (dirty working tree).
func IsDev() bool {
	return BuildDev == "true"
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

// CommitDateTime parses CommitDate (UTC RFC3339) into a time.Time in the local timezone.
func CommitDateTime() (time.Time, error) {
	if CommitDate == "unknown" || CommitDate == "" {
		return time.Time{}, fmt.Errorf("commit date unknown")
	}
	t, err := time.Parse(time.RFC3339, CommitDate)
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
