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
	s := BuildOS + "-" + BuildArch
	if BuildGUI != "" && BuildGUI != "unknown" {
		s += " gui=" + BuildGUI
	}
	if BuildBackend != "" {
		s += " backend=" + BuildBackend
	}
	if BuildCompiler != "" && BuildCompiler != "unknown" {
		s += " compiler=" + BuildCompiler
	}
	return s
}
