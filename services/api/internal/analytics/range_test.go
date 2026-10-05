package analytics

import (
	"testing"
	"time"
)

func TestSafeTimezone(t *testing.T) {
	if got := SafeTimezone("Africa/Addis_Ababa"); got != "Africa/Addis_Ababa" {
		t.Fatalf("got %q", got)
	}
	if got := SafeTimezone(""); got != "UTC" {
		t.Fatalf("got %q", got)
	}
	if got := SafeTimezone("not-a-zone'); DROP TABLE x;--"); got != "UTC" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveWindowUTC(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	window := ResolveWindow(now, "UTC", 7)

	if !window.Start.Equal(time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("start = %s", window.Start)
	}
	if !window.End.Equal(time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("end = %s", window.End)
	}
	if !window.PrevStart.Equal(time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("prev start = %s", window.PrevStart)
	}
	if !window.PrevEnd.Equal(window.Start) {
		t.Fatalf("prev end = %s, want %s", window.PrevEnd, window.Start)
	}
}

func TestResolveWindowRespectsProjectTimezone(t *testing.T) {
	// 2026-10-02 01:30 UTC is still 2026-10-01 in New York (EDT, UTC-4),
	// so the 1-day window must start at the New York midnight.
	now := time.Date(2026, 10, 2, 1, 30, 0, 0, time.UTC)
	window := ResolveWindow(now, "America/New_York", 1)

	newYork, _ := time.LoadLocation("America/New_York")
	wantStart := time.Date(2026, 10, 1, 0, 0, 0, 0, newYork).UTC()
	if !window.Start.Equal(wantStart) {
		t.Fatalf("start = %s, want %s", window.Start, wantStart)
	}
}

func TestDayLabels(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	labels := DayLabels(now, "Africa/Addis_Ababa", 3)
	want := []string{"2026-09-30", "2026-10-01", "2026-10-02"}
	if len(labels) != len(want) {
		t.Fatalf("labels = %v", labels)
	}
	for i := range want {
		if labels[i] != want[i] {
			t.Fatalf("labels = %v, want %v", labels, want)
		}
	}
}

func TestChangePct(t *testing.T) {
	change := ChangePct(120, 100)
	if change == nil || *change != 20 {
		t.Fatalf("change = %v", change)
	}
	change = ChangePct(50, 100)
	if change == nil || *change != -50 {
		t.Fatalf("change = %v", change)
	}
	if ChangePct(10, 0) != nil {
		t.Fatal("zero baseline must produce nil, not infinity")
	}
}

func TestValidateDays(t *testing.T) {
	for _, days := range []int{1, 7, 30, 90, 365} {
		if err := ValidateDays(days); err != nil {
			t.Fatalf("days=%d: %v", days, err)
		}
	}
	for _, days := range []int{0, -1, 366} {
		if err := ValidateDays(days); err == nil {
			t.Fatalf("days=%d must fail", days)
		} else if _, ok := err.(*ValidationError); !ok {
			t.Fatalf("days=%d: wrong error type %T", days, err)
		}
	}
}

func TestParseDate(t *testing.T) {
	parsed, err := ParseDate("2026-09-15")
	if err != nil {
		t.Fatalf("valid date rejected: %v", err)
	}
	if parsed.Year() != 2026 || parsed.Month() != time.September || parsed.Day() != 15 {
		t.Fatalf("parsed = %s", parsed)
	}
	for _, value := range []string{"", "2026-9-15", "09/15/2026", "2026-02-30", "not-a-date", "2026-09-15T00:00:00Z"} {
		if _, err := ParseDate(value); err == nil {
			t.Fatalf("ParseDate(%q) must fail", value)
		}
	}
}

func TestReportRangeValidate(t *testing.T) {
	valid := []ReportRange{
		{Days: 7},
		{Days: 1},
		{Days: 365},
		{Days: 7, EndDate: "2026-09-15"},
	}
	for _, rg := range valid {
		if err := rg.Validate(); err != nil {
			t.Fatalf("%+v: %v", rg, err)
		}
	}
	invalid := []ReportRange{
		{Days: 0},
		{Days: 366},
		{Days: -1},
		{Days: 7, EndDate: "2026-9-15"},
		{Days: 7, EndDate: "nonsense"},
	}
	for _, rg := range invalid {
		if err := rg.Validate(); err == nil {
			t.Fatalf("%+v must fail", rg)
		}
	}
}

func TestResolveWindowRangeMatchesPresetWithoutEndDate(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	preset := ResolveWindow(now, "Africa/Addis_Ababa", 7)
	rg := ResolveWindowRange(now, "Africa/Addis_Ababa", ReportRange{Days: 7})
	if !rg.Start.Equal(preset.Start) || !rg.End.Equal(preset.End) {
		t.Fatalf("range=%+v preset=%+v", rg, preset)
	}
	if !rg.PrevStart.Equal(preset.PrevStart) || !rg.PrevEnd.Equal(preset.PrevEnd) {
		t.Fatalf("prev range=%+v preset=%+v", rg, preset)
	}
}

func TestResolveWindowRangeCustomEndDate(t *testing.T) {
	// A custom window ends on the project-local end date regardless of now.
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	window := ResolveWindowRange(now, "Africa/Addis_Ababa", ReportRange{Days: 7, EndDate: "2026-09-15"})

	addis, _ := time.LoadLocation("Africa/Addis_Ababa")
	wantStart := time.Date(2026, 9, 9, 0, 0, 0, 0, addis).UTC()
	wantEnd := time.Date(2026, 9, 16, 0, 0, 0, 0, addis).UTC()
	if !window.Start.Equal(wantStart) {
		t.Fatalf("start = %s, want %s", window.Start, wantStart)
	}
	if !window.End.Equal(wantEnd) {
		t.Fatalf("end = %s, want %s", window.End, wantEnd)
	}
	if !window.PrevEnd.Equal(window.Start) {
		t.Fatalf("prev end = %s, want %s", window.PrevEnd, window.Start)
	}
	wantPrevStart := time.Date(2026, 9, 2, 0, 0, 0, 0, addis).UTC()
	if !window.PrevStart.Equal(wantPrevStart) {
		t.Fatalf("prev start = %s, want %s", window.PrevStart, wantPrevStart)
	}
}

func TestDayLabelsRangeCustomEndDate(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	labels := DayLabelsRange(now, "UTC", ReportRange{Days: 3, EndDate: "2026-09-15"})
	want := []string{"2026-09-13", "2026-09-14", "2026-09-15"}
	if len(labels) != len(want) {
		t.Fatalf("labels = %v", labels)
	}
	for i := range want {
		if labels[i] != want[i] {
			t.Fatalf("labels = %v, want %v", labels, want)
		}
	}
}

func TestDayLabelsRangeMatchesPresetWithoutEndDate(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	preset := DayLabels(now, "UTC", 5)
	rg := DayLabelsRange(now, "UTC", ReportRange{Days: 5})
	if len(preset) != len(rg) {
		t.Fatalf("labels = %v, preset = %v", rg, preset)
	}
	for i := range preset {
		if rg[i] != preset[i] {
			t.Fatalf("labels = %v, preset = %v", rg, preset)
		}
	}
}
