// Package metrics exposes a minimal Prometheus-text endpoint without
// external dependencies. Counters are process-local and reset on
// restart; that is sufficient for error-rate and throughput alerts
// until a dedicated telemetry stack lands.
package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
)

var registry struct {
	mu       sync.Mutex
	counters map[string]*atomic.Int64
}

func counter(name string) *atomic.Int64 {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if registry.counters == nil {
		registry.counters = map[string]*atomic.Int64{}
	}
	value, ok := registry.counters[name]
	if !ok {
		value = &atomic.Int64{}
		registry.counters[name] = value
	}
	return value
}

// AddIngested records an ingestion outcome: accepted, invalid,
// forbidden, not_found, rate_limited or error.
func AddIngested(outcome string) {
	counter(`events_ingested_total{outcome="` + outcome + `"}`).Add(1)
}

// AddReport records one dashboard report query by report name.
func AddReport(name string) {
	counter(`report_requests_total{report="` + name + `"}`).Add(1)
}

// AddRetentionRun records a retention cleanup pass.
func AddRetentionRun(failed bool, pageViews, sessions, visitors int64) {
	if failed {
		counter(`retention_runs_total{status="error"}`).Add(1)
		return
	}
	counter(`retention_runs_total{status="ok"}`).Add(1)
	counter(`retention_rows_deleted_total{kind="page_views"}`).Add(pageViews)
	counter(`retention_rows_deleted_total{kind="sessions"}`).Add(sessions)
	counter(`retention_rows_deleted_total{kind="visitors"}`).Add(visitors)
}

// Expose renders counters in Prometheus text exposition format.
func Expose(w http.ResponseWriter, _ *http.Request) {
	registry.mu.Lock()
	names := make([]string, 0, len(registry.counters))
	snapshot := make(map[string]int64, len(registry.counters))
	for name, value := range registry.counters {
		names = append(names, name)
		snapshot[name] = value.Load()
	}
	registry.mu.Unlock()
	sort.Strings(names)

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	for _, name := range names {
		fmt.Fprintf(w, "%s %d\n", name, snapshot[name])
	}
}
