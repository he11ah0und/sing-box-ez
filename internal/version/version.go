package version

import "fmt"

var (
	Version   = "dev"
	BuildDate = "unknown"
)

func Info() string {
	if BuildDate == "unknown" || BuildDate == "" {
		return Version
	}
	return fmt.Sprintf("%s (%s)", Version, BuildDate)
}
