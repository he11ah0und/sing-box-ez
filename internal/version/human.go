package version

import (
	"fmt"
	"time"

	"sing-box-ez/internal/i18n"
)

// HumanDuration returns a human-readable string like "2 hours ago" or "3 days ago"
// for the elapsed time since t. If t is zero, it returns an empty string.
func HumanDuration(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Minute:
		return i18n.T("duration.just_now")
	case d < time.Hour:
		return fmt.Sprintf(i18n.T("duration.minutes_ago"), int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf(i18n.T("duration.hours_ago"), int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf(i18n.T("duration.days_ago"), int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf(i18n.T("duration.months_ago"), int(d.Hours()/24/30))
	default:
		return fmt.Sprintf(i18n.T("duration.years_ago"), int(d.Hours()/24/365))
	}
}
