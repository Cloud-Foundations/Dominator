package ratelimit

import (
	"testing"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
)

// newTestLimiter builds a Limiter with metrics disabled so tests do not
// collide on the process-global tricorder registry.
func newTestLimiter(t *testing.T, limits Limits) *Limiter {
	t.Helper()
	limiter, err := NewLimiter(limits, Options{})
	if err != nil {
		t.Fatalf("NewLimiter: %s", err)
	}
	return limiter
}

// mustBeDenied asserts err is a *ResourceExhaustedError whose Reason equals
// wantLimitType. It fails the test otherwise.
func mustBeDenied(t *testing.T, err error, wantLimitType string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected denial with limit type %q, got nil", wantLimitType)
	}
	reErr, ok := err.(*errors.ResourceExhaustedError)
	if !ok {
		t.Fatalf("expected *ResourceExhaustedError, got %T: %v", err, err)
	}
	if reErr.Reason != wantLimitType {
		t.Fatalf("got Reason=%q; want %q", reErr.Reason, wantLimitType)
	}
}

func TestLimiter_AllowPathWithNoLimits(t *testing.T) {
	limiter := newTestLimiter(t, Limits{})
	for i := 0; i < 100; i++ {
		if err := limiter.Allow("Foo", "alice", ProtocolSRPC,
			false); err != nil {
			t.Fatalf("iteration %d: unexpected denial: %v", i, err)
		}
	}
}

func TestLimiter_NewRejectsBadBurst(t *testing.T) {
	cases := []Limits{
		{Global: MethodLimit{RequestsPerSecond: 1, Burst: 0}},
		{PerMethod: map[string]MethodLimit{
			"Foo": {RequestsPerSecond: 1, Burst: 0}}},
		{PerUserPerMethod: PerUserPerMethodLimits{
			Default: MethodLimit{RequestsPerSecond: 1, Burst: 0}}},
		{PerUserPerMethod: PerUserPerMethodLimits{
			Overrides: map[string]MethodLimit{
				"Foo": {RequestsPerSecond: 1, Burst: -1}}}},
	}
	for i, limits := range cases {
		if _, err := NewLimiter(limits, Options{}); err == nil {
			t.Fatalf("case %d: expected error, got nil", i)
		}
	}
}

func TestLimiter_GlobalDenial(t *testing.T) {
	limiter := newTestLimiter(t, Limits{
		Global: MethodLimit{RequestsPerSecond: 0.001, Burst: 2},
	})
	for i := 0; i < 2; i++ {
		if err := limiter.Allow("Foo", "alice", ProtocolGRPC,
			false); err != nil {
			t.Fatalf("burst request %d unexpectedly denied: %v", i, err)
		}
	}
	mustBeDenied(t,
		limiter.Allow("Foo", "alice", ProtocolGRPC, false),
		LimitTypeGlobal)
	if got := limiter.DeniedCount("Foo", LimitTypeGlobal,
		ProtocolGRPC); got != 1 {
		t.Fatalf("DeniedCount=%d; want 1", got)
	}
	// Global denials are not bypassed by bypassPerUser.
	if err := limiter.Allow("Foo", "admin", ProtocolGRPC, true); err == nil {
		t.Fatal("global limit must apply to bypassPerUser callers")
	}
}

func TestLimiter_PerMethodDenial(t *testing.T) {
	limiter := newTestLimiter(t, Limits{
		PerMethod: map[string]MethodLimit{
			"Expensive": {RequestsPerSecond: 0.001, Burst: 1},
		},
	})
	if err := limiter.Allow("Expensive", "alice", ProtocolREST,
		false); err != nil {
		t.Fatalf("first call unexpectedly denied: %v", err)
	}
	mustBeDenied(t,
		limiter.Allow("Expensive", "alice", ProtocolREST, false),
		LimitTypePerMethod)
	if err := limiter.Allow("Cheap", "alice", ProtocolREST,
		false); err != nil {
		t.Fatalf("unrelated method must not be limited: %v", err)
	}
}

func TestLimiter_PerUserPerMethodDenialAndIsolation(t *testing.T) {
	limiter := newTestLimiter(t, Limits{
		PerUserPerMethod: PerUserPerMethodLimits{
			Default: MethodLimit{RequestsPerSecond: 0.001, Burst: 1},
		},
	})
	if err := limiter.Allow("Foo", "alice", ProtocolSRPC,
		false); err != nil {
		t.Fatalf("alice's first call denied: %v", err)
	}
	mustBeDenied(t,
		limiter.Allow("Foo", "alice", ProtocolSRPC, false),
		LimitTypePerUserPerMethod)
	// Distinct user has its own bucket.
	if err := limiter.Allow("Foo", "bob", ProtocolSRPC, false); err != nil {
		t.Fatalf("bob's first call denied: %v", err)
	}
	// Distinct method shares the default config but a separate bucket.
	if err := limiter.Allow("Bar", "alice", ProtocolSRPC, false); err != nil {
		t.Fatalf("alice's call to a different method denied: %v", err)
	}
	// Unauthenticated requests skip the per-user tier entirely.
	for i := 0; i < 10; i++ {
		if err := limiter.Allow("Foo", "", ProtocolSRPC, false); err != nil {
			t.Fatalf("unauthenticated request %d denied: %v", i, err)
		}
	}
	// bypassPerUser skips the per-user tier.
	for i := 0; i < 10; i++ {
		if err := limiter.Allow("Foo", "alice", ProtocolSRPC,
			true); err != nil {
			t.Fatalf("bypassPerUser request %d denied: %v", i, err)
		}
	}
}
