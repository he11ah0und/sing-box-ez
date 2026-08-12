package version

import (
	"time"

	"github.com/he11ah0und/localengine"
)

// HumanDuration returns a human-readable string like "2 hours ago" or "3 days ago"
// for the elapsed time since t. If t is zero, it returns an empty string.
func HumanDuration(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return HumanDurationFrom(time.Since(t), false)
}

// HumanDurationFrom formats a duration into a human-readable string.
// If future is true, uses "in X min" style; otherwise "X min ago".
func HumanDurationFrom(d time.Duration, future bool) string {
	if d < 0 {
		d = -d
	}
	if d < time.Minute {
		return localengine.T("duration", "just_now")
	}

	unit := humanDurationUnit(d)
	if future {
		return localengine.Tf(localengine.Vars{"unit": unit}, "duration", "in")
	}
	return localengine.Tf(localengine.Vars{"unit": unit}, "duration", "ago")
}

// HumanDurationPlain returns a compact numeric duration like "5s", "3h 5m", "1d",
// without "ago", "just now" or other suffixes.
func HumanDurationPlain(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	return humanDurationUnit(d)
}

func humanDurationUnit(d time.Duration) string {
	if d < time.Minute {
		return localengine.Tf(localengine.Vars{"count": int(d.Seconds())}, "duration", "seconds")
	}
	switch {
	case d < time.Hour:
		return localengine.Tf(localengine.Vars{"count": int(d.Minutes())}, "duration", "minutes")
	case d < 24*time.Hour:
		hours := int(d.Hours())
		mins := int(d.Minutes()) % 60
		hoursStr := localengine.Tf(localengine.Vars{"count": hours}, "duration", "hours")
		if mins == 0 {
			return hoursStr
		}
		minsStr := localengine.Tf(localengine.Vars{"count": mins}, "duration", "minutes")
		return hoursStr + " " + minsStr
	case d < 30*24*time.Hour:
		return localengine.Tf(localengine.Vars{"count": int(d.Hours() / 24)}, "duration", "days")
	case d < 365*24*time.Hour:
		return localengine.Tf(localengine.Vars{"count": int(d.Hours() / 24 / 30)}, "duration", "months")
	default:
		return localengine.Tf(localengine.Vars{"count": int(d.Hours() / 24 / 365)}, "duration", "years")
	}
}
