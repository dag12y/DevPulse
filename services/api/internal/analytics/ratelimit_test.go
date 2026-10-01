package analytics

import (
	"testing"
	"time"
)

func TestLimiterAllowsBurstThenDenies(t *testing.T) {
	limiter := NewLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		if !limiter.Allow("dp_test", "10.0.0.1") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if limiter.Allow("dp_test", "10.0.0.1") {
		t.Fatal("request over the limit should be denied")
	}
}

func TestLimiterScopesByProjectAndIP(t *testing.T) {
	limiter := NewLimiter(1, time.Minute)
	if !limiter.Allow("dp_a", "10.0.0.1") {
		t.Fatal("first request should be allowed")
	}
	if !limiter.Allow("dp_b", "10.0.0.1") {
		t.Fatal("different project should have its own bucket")
	}
	if !limiter.Allow("dp_a", "10.0.0.2") {
		t.Fatal("different IP should have its own bucket")
	}
}

func TestLimiterWindowResets(t *testing.T) {
	now := time.Now()
	limiter := NewLimiter(1, time.Minute)
	limiter.now = func() time.Time { return now }
	if !limiter.Allow("dp_test", "10.0.0.1") {
		t.Fatal("first request should be allowed")
	}
	now = now.Add(61 * time.Second)
	if !limiter.Allow("dp_test", "10.0.0.1") {
		t.Fatal("request in a new window should be allowed")
	}
}
