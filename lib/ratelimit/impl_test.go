package ratelimit

import (
	"fmt"
	"testing"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
)

const usersPerWave = 5000

// Metrics are disabled: the tricorder registry is process-global.
func newTestLimiter(t *testing.T, limits Limits) *Limiter {
	t.Helper()
	limiter, err := New(limits, Options{})
	if err != nil {
		t.Fatalf("New: %s", err)
	}
	return limiter
}

func mustBeDenied(t *testing.T, err error, wantLimitType LimitType) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected denial with limit type %s, got nil", wantLimitType)
	}
	reErr, ok := err.(*errors.ResourceExhaustedError)
	if !ok {
		t.Fatalf("expected *ResourceExhaustedError, got %T: %v", err, err)
	}
	if reErr.Reason != wantLimitType.String() {
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
		if _, err := New(limits, Options{}); err == nil {
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

// Exercises the registration path in recordDenial. The subdirectory name is
// unique to this test because the tricorder registry is process-global.
func TestLimiter_MetricsRegistration(t *testing.T) {
	limiter, err := New(
		Limits{Global: MethodLimit{RequestsPerSecond: 0.001, Burst: 1}},
		Options{MetricsSubDirname: "test/ratelimit/metrics"})
	if err != nil {
		t.Fatalf("New: %s", err)
	}
	if err := limiter.Allow("Foo", "alice", ProtocolGRPC, false); err != nil {
		t.Fatalf("first call unexpectedly denied: %v", err)
	}
	mustBeDenied(t, limiter.Allow("Foo", "alice", ProtocolGRPC, false),
		LimitTypeGlobal)
	// Must reuse the counter, not re-register it.
	mustBeDenied(t, limiter.Allow("Foo", "alice", ProtocolGRPC, false),
		LimitTypeGlobal)
	if got := limiter.DeniedCount("Foo", LimitTypeGlobal,
		ProtocolGRPC); got != 2 {
		t.Fatalf("DeniedCount=%d; want 2", got)
	}
}

// Two Limiters sharing a metrics directory: the second collides with the
// first, which must not panic on the request path.
func TestLimiter_MetricsRegistrationFailureIsNotFatal(t *testing.T) {
	newSharing := func() *Limiter {
		limiter, err := New(
			Limits{Global: MethodLimit{RequestsPerSecond: 0.001, Burst: 1}},
			Options{MetricsSubDirname: "test/ratelimit/shared"})
		if err != nil {
			t.Fatalf("New: %s", err)
		}
		return limiter
	}
	first, second := newSharing(), newSharing()
	if err := first.Allow("Foo", "alice", ProtocolSRPC, false); err != nil {
		t.Fatalf("first call unexpectedly denied: %v", err)
	}
	mustBeDenied(t, first.Allow("Foo", "alice", ProtocolSRPC, false),
		LimitTypeGlobal)
	// The second Limiter registers the same path; denials still count.
	if err := second.Allow("Foo", "alice", ProtocolSRPC, false); err != nil {
		t.Fatalf("second limiter's first call unexpectedly denied: %v", err)
	}
	mustBeDenied(t, second.Allow("Foo", "alice", ProtocolSRPC, false),
		LimitTypeGlobal)
	if got := second.DeniedCount("Foo", LimitTypeGlobal,
		ProtocolSRPC); got != 1 {
		t.Fatalf("DeniedCount=%d; want 1", got)
	}
}

// A wave arriving faster than its buckets refill is fully retained, and is
// reclaimed only once a later wave drives sweeps.
func TestLimiter_ReclaimsIdleBuckets(t *testing.T) {
	// Burst of 1 at 100 rps replenishes in 10ms, keeping the test quick.
	limiter := newTestLimiter(t, Limits{
		PerUserPerMethod: PerUserPerMethodLimits{
			Default: MethodLimit{RequestsPerSecond: 100, Burst: 1},
		},
	})
	size := func() int {
		limiter.mutex.Lock()
		defer limiter.mutex.Unlock()
		return len(limiter.perUser)
	}
	wave := func(name string) {
		for i := 0; i < usersPerWave; i++ {
			if err := limiter.Allow("Foo", fmt.Sprintf("%s-%d", name, i),
				ProtocolSRPC, false); err != nil {
				t.Fatalf("%s-%d denied: %v", name, i, err)
			}
		}
	}
	wave("first")
	// The second wave's insertions drive the sweeps.
	time.Sleep(50 * time.Millisecond)
	wave("second")
	if retained := size(); retained > usersPerWave*3/2 {
		t.Fatalf("retained %d buckets after %d users across two waves: the "+
			"first wave was not reclaimed", retained, usersPerWave*2)
	} else {
		t.Logf("retained %d buckets after %d users across two waves",
			retained, usersPerWave*2)
	}
}

// A user who has spent their tokens must stay limited however many other
// users churn through the map meanwhile.
func TestLimiter_ReclamationDoesNotBypassLimit(t *testing.T) {
	limiter := newTestLimiter(t, Limits{
		PerUserPerMethod: PerUserPerMethodLimits{
			Default: MethodLimit{RequestsPerSecond: 0.001, Burst: 1},
		},
	})
	if err := limiter.Allow("Foo", "victim", ProtocolSRPC, false); err != nil {
		t.Fatalf("victim's first call denied: %v", err)
	}
	mustBeDenied(t, limiter.Allow("Foo", "victim", ProtocolSRPC, false),
		LimitTypePerUserPerMethod)
	// Churn many other users through the map, driving reclamation sweeps.
	for i := 0; i < 5000; i++ {
		if err := limiter.Allow("Foo", fmt.Sprintf("other-%d", i),
			ProtocolSRPC, false); err != nil {
			t.Fatalf("other-%d denied: %v", i, err)
		}
	}
	// The drained bucket is not full, so not reclaimable.
	mustBeDenied(t, limiter.Allow("Foo", "victim", ProtocolSRPC, false),
		LimitTypePerUserPerMethod)
}

// The names are a wire contract: they appear in tricorder metric paths and in
// the limit_type field of denial logs.
func TestLimitTypeString(t *testing.T) {
	expected := map[LimitType]string{
		LimitTypeGlobal:           "global",
		LimitTypePerMethod:        "per_method",
		LimitTypePerUserPerMethod: "per_user_per_method",
	}
	for limitType, want := range expected {
		if got := limitType.String(); got != want {
			t.Errorf("LimitType(%d).String()=%q; want %q", limitType, got,
				want)
		}
	}
	if len(limitTypeToText) != len(expected) {
		t.Errorf("limitTypeToText has %d entries; want %d",
			len(limitTypeToText), len(expected))
	}
	if got := LimitType(99).String(); got != limitTypeUnknown {
		t.Errorf("got %q; want %q", got, limitTypeUnknown)
	}
}

func TestLimitTypeTextRoundTrip(t *testing.T) {
	for limitType := range limitTypeToText {
		text, err := limitType.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText(%s): %s", limitType, err)
		}
		var decoded LimitType
		if err := decoded.UnmarshalText(text); err != nil {
			t.Fatalf("UnmarshalText(%q): %s", text, err)
		}
		if decoded != limitType {
			t.Fatalf("round trip gave %s; want %s", decoded, limitType)
		}
	}
	if _, err := LimitType(99).MarshalText(); err == nil {
		t.Fatal("MarshalText of an unknown LimitType must fail")
	}
	var limitType LimitType
	if err := limitType.UnmarshalText([]byte("bogus")); err == nil {
		t.Fatal("UnmarshalText of an unknown name must fail")
	}
}

func TestProtocolString(t *testing.T) {
	expected := map[Protocol]string{
		ProtocolSRPC: "srpc",
		ProtocolGRPC: "grpc",
		ProtocolREST: "rest",
	}
	for protocol, want := range expected {
		if got := protocol.String(); got != want {
			t.Errorf("Protocol(%d).String()=%q; want %q", protocol, got, want)
		}
	}
	if len(protocolToText) != len(expected) {
		t.Errorf("protocolToText has %d entries; want %d", len(protocolToText),
			len(expected))
	}
	if got := Protocol(99).String(); got != protocolUnknown {
		t.Errorf("got %q; want %q", got, protocolUnknown)
	}
}

func TestProtocolTextRoundTrip(t *testing.T) {
	for protocol := range protocolToText {
		text, err := protocol.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText(%s): %s", protocol, err)
		}
		var decoded Protocol
		if err := decoded.UnmarshalText(text); err != nil {
			t.Fatalf("UnmarshalText(%q): %s", text, err)
		}
		if decoded != protocol {
			t.Fatalf("round trip gave %s; want %s", decoded, protocol)
		}
	}
	if _, err := Protocol(99).MarshalText(); err == nil {
		t.Fatal("MarshalText of an unknown Protocol must fail")
	}
	var protocol Protocol
	if err := protocol.UnmarshalText([]byte("bogus")); err == nil {
		t.Fatal("UnmarshalText of an unknown name must fail")
	}
}
