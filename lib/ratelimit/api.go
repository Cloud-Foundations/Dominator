/*
Package ratelimit implements a three-tier token-bucket rate limiter intended to
be shared across SRPC, gRPC and REST handlers in a single process so that
quotas cannot be bypassed by switching protocols.

The three tiers, checked in order, guard against distinct failure modes:

  - global: caps total admitted requests per second across all users and
    methods, as a server-wide safety net.
  - per-method: caps total requests per second for a given method across all
    users, configured for expensive operations whose aggregate cost matters
    even when no single user is misbehaving.
  - per-user-per-method: caps how often a single identified user may invoke a
    given method, with a default that applies to every (user, method) pair and
    optional per-method overrides.

The Limiter exposes a protocol-neutral Allow method. Adapters in
protocol-specific packages (e.g. lib/srpc/serverutil.RateLimiterBlocker)
plug it into the protocol's request pipeline.
*/
package ratelimit

import (
	"sync"

	"github.com/Cloud-Foundations/tricorder/go/tricorder"
	"golang.org/x/time/rate"

	"github.com/Cloud-Foundations/Dominator/lib/log"
)

// Protocol identifies the wire protocol that admitted a request. It is used as
// a dimension on rate-limit denial metrics so that quota violations can be
// attributed to SRPC, gRPC or REST.
type Protocol string

const (
	ProtocolSRPC Protocol = "srpc"
	ProtocolGRPC Protocol = "grpc"
	ProtocolREST Protocol = "rest"
)

// Limit-type names recorded on rate-limit denials. They form the second path
// component of the denial metric tree and the limit_type field of denial logs.
const (
	LimitTypeGlobal           = "global"
	LimitTypePerMethod        = "per_method"
	LimitTypePerUserPerMethod = "per_user_per_method"
)

// MethodLimit describes a token-bucket rate limit. A non-positive
// RequestsPerSecond disables the limit. Burst must be positive whenever
// RequestsPerSecond is positive.
type MethodLimit struct {
	RequestsPerSecond float64 `json:"requests_per_second"`
	Burst             int     `json:"burst"`
}

// PerUserPerMethodLimits configures the per-user-per-method tier. Default
// applies to every (user, method) pair that does not appear in Overrides.
type PerUserPerMethodLimits struct {
	Default   MethodLimit            `json:"default"`
	Overrides map[string]MethodLimit `json:"overrides,omitempty"`
}

// Limits is the top-level rate-limiting configuration consumed by NewLimiter.
// A zero-valued tier is treated as unlimited.
type Limits struct {
	Global           MethodLimit            `json:"global"`
	PerMethod        map[string]MethodLimit `json:"per_method,omitempty"`
	PerUserPerMethod PerUserPerMethodLimits `json:"per_user_per_method"`
}

// Options configures non-policy aspects of a Limiter.
type Options struct {
	// Logger receives debug-level denial messages. May be nil.
	Logger log.DebugLogger
	// MetricsDirectoryName is the tricorder directory under which
	// per-(method, limit_type, protocol) denial counters are registered.
	// If empty, no tricorder metrics are registered (counters are still
	// maintained internally and visible via DeniedCount).
	MetricsDirectoryName string
}

// Limiter is a three-tier token-bucket rate limiter. Methods are safe for
// concurrent use.
type Limiter struct {
	logger     log.DebugLogger
	global     *rate.Limiter
	perMethod  map[string]*rate.Limiter
	perUserCfg PerUserPerMethodLimits

	mutex   sync.Mutex
	perUser map[userMethodType]*rate.Limiter

	countersMu sync.Mutex
	counters   map[denialKey]*uint64
	metricsDir *tricorder.DirectorySpec
}

type userMethodType struct {
	method   string
	username string
}

type denialKey struct {
	method    string
	limitType string
	protocol  Protocol
}

// NewLimiter constructs a Limiter from the supplied configuration. It returns
// an error if any tier has a positive RequestsPerSecond but a non-positive
// Burst, or if metric registration fails.
func NewLimiter(limits Limits, opts Options) (*Limiter, error) {
	return newLimiter(limits, opts)
}

// Allow tests a single request against all three tiers in order: global,
// per-method, per-user-per-method. It returns nil if the request is admitted.
// On denial it returns a *errors.ResourceExhaustedError whose Resource is the
// method name and whose Reason is one of LimitTypeGlobal, LimitTypePerMethod
// or LimitTypePerUserPerMethod; the matching denial counter is incremented
// and a debug log line is emitted if a logger is set. Adapters can forward
// the error directly: it maps to gRPC codes.ResourceExhausted via GrpcCode()
// and to the corresponding REST status via grpc-gateway.
//
// username may be empty for unauthenticated callers; the per-user-per-method
// tier is then skipped (the global and per-method tiers still apply). When
// bypassPerUser is true (e.g. the caller has elevated method-power access)
// the per-user-per-method tier is skipped regardless of username.
func (l *Limiter) Allow(method, username string, protocol Protocol,
	bypassPerUser bool) error {
	return l.allow(method, username, protocol, bypassPerUser)
}

// DeniedCount returns the total number of denials recorded for the given
// (method, limitType, protocol) triple. It is safe to call concurrently.
func (l *Limiter) DeniedCount(method, limitType string,
	protocol Protocol) uint64 {
	return l.deniedCount(method, limitType, protocol)
}
