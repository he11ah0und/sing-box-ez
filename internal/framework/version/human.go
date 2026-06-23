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

	var unit string
	switch {
	case d < time.Hour:
		unit = fmt.Sprintf(localengine.T("duration", "unit_minutes"), int(d.Minutes()))
	case d < 24*time.Hour:
		hours := int(d.Hours())
		mins := int(d.Minutes()) % 60
		hoursStr := fmt.Sprintf(localengine.T("duration", "unit_hours"), hours)
		if mins == 0 {
			unit = hoursStr
		} else {
			minsStr := fmt.Sprintf(localengine.T("duration", "unit_minutes"), mins)
			unit = hoursStr + " " + minsStr
		}
	case d < 30*24*time.Hour:
		unit = fmt.Sprintf(localengine.T("duration", "unit_days"), int(d.Hours()/24))
	case d < 365*24*time.Hour:
		unit = fmt.Sprintf(localengine.T("duration", "unit_months"), int(d.Hours()/24/30))
	default:
		unit = fmt.Sprintf(localengine.T("duration", "unit_years"), int(d.Hours()/24/365))
	}
	if future {
		return fmt.Sprintf(localengine.T("duration", "in"), unit)
	}
	return fmt.Sprintf(localengine.T("duration", "ago"), unit)
}
