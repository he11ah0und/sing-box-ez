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

	var unitKey string
	var value int
	switch {
	case d < time.Hour:
		unitKey = "duration.unit_minutes"
		value = int(d.Minutes())
	case d < 24*time.Hour:
		unitKey = "duration.unit_hours"
		value = int(d.Hours())
	case d < 30*24*time.Hour:
		unitKey = "duration.unit_days"
		value = int(d.Hours() / 24)
	case d < 365*24*time.Hour:
		unitKey = "duration.unit_months"
		value = int(d.Hours() / 24 / 30)
	default:
		unitKey = "duration.unit_years"
		value = int(d.Hours() / 24 / 365)
	}

	unit := fmt.Sprintf(localengine.T(unitKey), value)
	if future {
		return fmt.Sprintf(localengine.T("duration", "in"), unit)
	}
	return fmt.Sprintf(localengine.T("duration", "ago"), unit)
}
