package analytics

import (
	"fmt"
	"time"
)

// Reporting window helpers. All dashboard aggregates (except realtime)
// operate on whole project-local days: the current window covers today
// plus the previous days-1 days in the project's timezone, and the
// previous window covers the days immediately before that.

const (
	// DefaultDays is used when the caller omits ?days=.
	DefaultDays = 30
	// MaxReportDays bounds every reporting window.
	MaxReportDays = 365
)

// Window holds UTC bounds for the current and previous reporting windows.
// Bounds are half-open: [Start, End) and [PrevStart, PrevEnd).
type Window struct {
	Start     time.Time
	End       time.Time
	PrevStart time.Time
	PrevEnd   time.Time
}

// SafeTimezone returns name when it is a valid IANA timezone, else UTC.
// Project timezones are validated on write; this is defense in depth so a
// bad value can never break a dashboard query or reach SQL unchecked.
func SafeTimezone(name string) string {
	if name == "" {
		return "UTC"
	}
	if _, err := time.LoadLocation(name); err != nil {
		return "UTC"
	}
	return name
}

// ResolveWindow computes UTC bounds for a days-long window ending today
// in the given timezone. now is the reference instant (usually time.Now).
func ResolveWindow(now time.Time, timezone string, days int) Window {
	if days < 1 {
		days = 1
	}
	location, err := time.LoadLocation(SafeTimezone(timezone))
	if err != nil {
		location = time.UTC
	}
	localNow := now.In(location)
	midnight := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
	end := midnight.AddDate(0, 0, 1)
	start := midnight.AddDate(0, 0, -(days - 1))
	return Window{
		Start:     start.UTC(),
		End:       end.UTC(),
		PrevStart: start.AddDate(0, 0, -days).UTC(),
		PrevEnd:   start.UTC(),
	}
}

// DayLabels returns project-local YYYY-MM-DD labels for the current
// window, oldest first. Used to fill traffic days that have no events.
func DayLabels(now time.Time, timezone string, days int) []string {
	if days < 1 {
		days = 1
	}
	location, err := time.LoadLocation(SafeTimezone(timezone))
	if err != nil {
		location = time.UTC
	}
	localNow := now.In(location)
	midnight := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
	labels := make([]string, 0, days)
	for i := days - 1; i >= 0; i-- {
		labels = append(labels, midnight.AddDate(0, 0, -i).Format("2006-01-02"))
	}
	return labels
}

// ChangePct returns the relative percent change from prev to current,
// or nil when prev is zero (no meaningful baseline). Callers render nil
// as an em dash rather than inventing a number.
func ChangePct(current, prev float64) *float64 {
	if prev == 0 {
		return nil
	}
	change := (current - prev) / prev * 100
	return &change
}

// ValidateDays rejects out-of-range reporting windows.
func ValidateDays(days int) error {
	if days < 1 || days > MaxReportDays {
		return &ValidationError{err: fmt.Errorf("days must be between 1 and %d", MaxReportDays)}
	}
	return nil
}
