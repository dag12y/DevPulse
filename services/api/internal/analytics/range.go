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
	return windowEndingAt(midnight, days)
}

// windowEndingAt builds the half-open window whose last project-local day
// ends at endMidnight (the midnight that starts the following day).
func windowEndingAt(endMidnight time.Time, days int) Window {
	end := endMidnight.AddDate(0, 0, 1)
	start := endMidnight.AddDate(0, 0, -(days - 1))
	return Window{
		Start:     start.UTC(),
		End:       end.UTC(),
		PrevStart: start.AddDate(0, 0, -days).UTC(),
		PrevEnd:   start.UTC(),
	}
}

// ReportRange describes a reporting window: a length in whole project-local
// days plus an optional last day (YYYY-MM-DD in the project timezone). An
// empty EndDate means "ending today", which preserves the ?days= behavior.
type ReportRange struct {
	Days    int    `json:"days"`
	EndDate string `json:"end_date,omitempty"`
}

// ParseDate parses a strict YYYY-MM-DD date, rejecting other layouts so a
// malformed value can never silently shift a window.
func ParseDate(value string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil || parsed.Format("2006-01-02") != value {
		return time.Time{}, fmt.Errorf("date must be in YYYY-MM-DD format")
	}
	return parsed, nil
}

// Validate rejects out-of-range or inconsistent reporting windows.
func (rg ReportRange) Validate() error {
	if err := ValidateDays(rg.Days); err != nil {
		return err
	}
	if rg.EndDate != "" {
		if _, err := ParseDate(rg.EndDate); err != nil {
			return &ValidationError{err: err}
		}
	}
	return nil
}

// ResolveWindowRange resolves the window for a ReportRange in the given
// timezone. now only anchors windows that end today.
func ResolveWindowRange(now time.Time, timezone string, rg ReportRange) Window {
	if rg.EndDate == "" {
		return ResolveWindow(now, timezone, rg.Days)
	}
	location, err := time.LoadLocation(SafeTimezone(timezone))
	if err != nil {
		location = time.UTC
	}
	end, err := ParseDate(rg.EndDate)
	if err != nil {
		return ResolveWindow(now, timezone, rg.Days)
	}
	midnight := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, location)
	return windowEndingAt(midnight, rg.Days)
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
	return labelsEndingAt(midnight, days)
}

// DayLabelsRange returns labels for a ReportRange, oldest first.
func DayLabelsRange(now time.Time, timezone string, rg ReportRange) []string {
	if rg.EndDate == "" {
		return DayLabels(now, timezone, rg.Days)
	}
	location, err := time.LoadLocation(SafeTimezone(timezone))
	if err != nil {
		location = time.UTC
	}
	end, err := ParseDate(rg.EndDate)
	if err != nil {
		return DayLabels(now, timezone, rg.Days)
	}
	midnight := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, location)
	return labelsEndingAt(midnight, rg.Days)
}

// labelsEndingAt emits days labels walking back from endMidnight.
func labelsEndingAt(endMidnight time.Time, days int) []string {
	labels := make([]string, 0, days)
	for i := days; i >= 1; i-- {
		labels = append(labels, endMidnight.AddDate(0, 0, -i+1).Format("2006-01-02"))
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
