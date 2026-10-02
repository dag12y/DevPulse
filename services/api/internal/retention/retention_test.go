package retention

import (
	"testing"
	"time"
)

func TestCutoff(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	if got := Cutoff(now, 30); !got.Equal(time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("cutoff = %s", got)
	}
	if got := Cutoff(now, 365); !got.Equal(time.Date(2025, 10, 2, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("cutoff = %s", got)
	}
}
