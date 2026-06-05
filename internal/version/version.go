package version

import "fmt"

var (
	Version   = "dev"
	BuildDate = "unknown"
	Commit    = "unknown"
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
