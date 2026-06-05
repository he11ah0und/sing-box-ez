package version

import "fmt"

var (
	Version   = "dev"
	BuildDate = "unknown"
	Commit    = "unknown"
	RepoURL   = "https://github.com/he11ah0und/sing-box-ez"

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
			return Version
		}
		return fmt.Sprintf("%s (%s)", Version, Commit)
	case Commit == "unknown" || Commit == "":
		return fmt.Sprintf("%s (%s)", Version, BuildDate)
	default:
		return fmt.Sprintf("%s (%s %s)", Version, BuildDate, Commit)
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
