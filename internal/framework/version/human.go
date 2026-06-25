package version

import (
	"fmt"
	"time"

	"sing-box-ez/internal/framework/localengine"
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
		return fmt.Sprintf(localengine.T("duration", "in"), unit)
	}
	return fmt.Sprintf(localengine.T("duration", "ago"), unit)
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
		return fmt.Sprintf(localengine.T("duration", "seconds"), int(d.Seconds()))
	}
	switch {
	case d < time.Hour:
		return fmt.Sprintf(localengine.T("duration", "minutes"), int(d.Minutes()))
	case d < 24*time.Hour:
		hours := int(d.Hours())
		mins := int(d.Minutes()) % 60
		hoursStr := fmt.Sprintf(localengine.T("duration", "hours"), hours)
		if mins == 0 {
			return hoursStr
		}
		minsStr := fmt.Sprintf(localengine.T("duration", "minutes"), mins)
		return hoursStr + " " + minsStr
	case d < 30*24*time.Hour:
		return fmt.Sprintf(localengine.T("duration", "days"), int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf(localengine.T("duration", "months"), int(d.Hours()/24/30))
	default:
		return fmt.Sprintf(localengine.T("duration", "years"), int(d.Hours()/24/365))
	}
}
