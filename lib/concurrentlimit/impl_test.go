package concurrentlimit

import (
	"testing"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
)

func TestLimiter_UnlimitedWhenNoConfig(t *testing.T) {
	limiter := NewLimiter(nil)
	for i := 0; i < 100; i++ {
		release, err := limiter.Acquire("Foo", "alice", false)
		if err != nil {
			t.Fatalf("iteration %d: unexpected denial: %v", i, err)
		}
		if release != nil {
			t.Fatalf("iteration %d: expected nil release for unlimited method",
				i)
		}
	}
}

func TestLimiter_ZeroLimitIsUnlimited(t *testing.T) {
	limiter := NewLimiter(Limits{"Foo": 0})
	for i := 0; i < 10; i++ {
		release, err := limiter.Acquire("Foo", "alice", false)
		if err != nil {
			t.Fatalf("iteration %d: unexpected denial: %v", i, err)
		}
		if release != nil {
			t.Fatal("expected nil release for zero-limit method")
		}
	}
}

func TestLimiter_AcquireDenyRelease(t *testing.T) {
	limiter := NewLimiter(Limits{"Foo": 2})
	first, err := limiter.Acquire("Foo", "alice", false)
	if err != nil || first == nil {
		t.Fatalf("first Acquire: (release=%t, err=%v)", first != nil, err)
	}
	second, err := limiter.Acquire("Foo", "alice", false)
	if err != nil || second == nil {
		t.Fatalf("second Acquire: (release=%t, err=%v)", second != nil, err)
	}
	// Third exceeds the limit.
	third, err := limiter.Acquire("Foo", "alice", false)
	if err == nil {
		t.Fatal("third Acquire: expected denial, got nil")
	}
	if third != nil {
		t.Fatal("third Acquire: expected nil release on denial")
	}
	reErr, ok := err.(*errors.ResourceExhaustedError)
	if !ok {
		t.Fatalf("expected *ResourceExhaustedError, got %T: %v", err, err)
	}
	if reErr.Resource != "Foo" {
		t.Fatalf("Resource=%q; want %q", reErr.Resource, "Foo")
	}
	// After releasing one slot, another Acquire admits.
	first()
	fourth, err := limiter.Acquire("Foo", "alice", false)
	if err != nil || fourth == nil {
		t.Fatalf("post-release Acquire: (release=%t, err=%v)", fourth != nil, err)
	}
	second()
	fourth()
}

func TestLimiter_UsersAreIndependent(t *testing.T) {
	limiter := NewLimiter(Limits{"Foo": 1})
	aliceRel, err := limiter.Acquire("Foo", "alice", false)
	if err != nil || aliceRel == nil {
		t.Fatalf("alice Acquire: (release=%t, err=%v)", aliceRel != nil, err)
	}
	// Bob has his own slot.
	bobRel, err := limiter.Acquire("Foo", "bob", false)
	if err != nil || bobRel == nil {
		t.Fatalf("bob Acquire: (release=%t, err=%v)", bobRel != nil, err)
	}
	// Alice's second call is denied while she still holds the slot.
	if _, err := limiter.Acquire("Foo", "alice", false); err == nil {
		t.Fatal("expected alice's second Acquire to deny")
	}
	aliceRel()
	bobRel()
}

func TestLimiter_MethodsAreIndependent(t *testing.T) {
	limiter := NewLimiter(Limits{"Foo": 1, "Bar": 1})
	fooRel, err := limiter.Acquire("Foo", "alice", false)
	if err != nil || fooRel == nil {
		t.Fatalf("Foo Acquire: (release=%t, err=%v)", fooRel != nil, err)
	}
	// Different method has its own slot for the same user.
	barRel, err := limiter.Acquire("Bar", "alice", false)
	if err != nil || barRel == nil {
		t.Fatalf("Bar Acquire: (release=%t, err=%v)", barRel != nil, err)
	}
	fooRel()
	barRel()
}

func TestLimiter_BypassAdmitsWithoutCounting(t *testing.T) {
	limiter := NewLimiter(Limits{"Foo": 1})
	for i := 0; i < 10; i++ {
		release, err := limiter.Acquire("Foo", "admin", true)
		if err != nil {
			t.Fatalf("iteration %d: bypass call denied: %v", i, err)
		}
		if release != nil {
			t.Fatal("bypass call should return nil release")
		}
	}
	// Non-bypass calls for the same user still see an empty counter.
	release, err := limiter.Acquire("Foo", "admin", false)
	if err != nil || release == nil {
		t.Fatalf("post-bypass Acquire: (release=%t, err=%v)", release != nil, err)
	}
	release()
}

func TestLimiter_ReleasePanicsOnOverRelease(t *testing.T) {
	limiter := NewLimiter(Limits{"Foo": 1})
	release, err := limiter.Acquire("Foo", "alice", false)
	if err != nil || release == nil {
		t.Fatalf("Acquire: (release=%t, err=%v)", release != nil, err)
	}
	release()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on over-release, got none")
		}
	}()
	release()
}

func TestLimiter_ConstructorCopiesLimits(t *testing.T) {
	original := Limits{"Foo": 1}
	limiter := NewLimiter(original)
	// Mutating the caller's map after construction must not affect the
	// limiter.
	original["Foo"] = 100
	release, err := limiter.Acquire("Foo", "alice", false)
	if err != nil || release == nil {
		t.Fatalf("first Acquire: (release=%t, err=%v)", release != nil, err)
	}
	if _, err := limiter.Acquire("Foo", "alice", false); err == nil {
		t.Fatal("expected denial (original limit was 1, mutation should not have leaked)")
	}
	release()
}
