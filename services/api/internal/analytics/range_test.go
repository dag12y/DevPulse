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
