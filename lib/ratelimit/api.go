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

A single Limiter is shared by every protocol served from one process, so its
buckets and denial counters are common to all of them.
*/
package ratelimit

import (
	"sync"

	"github.com/Cloud-Foundations/tricorder/go/tricorder"
	"golang.org/x/time/rate"

	"github.com/Cloud-Foundations/Dominator/lib/log"
)

// LimitType identifies the tier which denied a request. It forms the second
// path component of the denial metric tree and the limit_type field of denial
// logs.
type LimitType uint

// Protocol identifies the wire protocol which carried a request. It is used as
// a dimension on rate-limit denial metrics so that quota violations can be
// attributed to SRPC, gRPC or REST.
type Protocol uint

const (
	LimitTypeGlobal           = LimitType(0)
	LimitTypePerMethod        = LimitType(1)
	LimitTypePerUserPerMethod = LimitType(2)

	ProtocolSRPC = Protocol(0)
	ProtocolGRPC = Protocol(1)
	ProtocolREST = Protocol(2)
)

func (limitType LimitType) MarshalText() ([]byte, error) {
	return limitType.marshalText()
}

func (limitType LimitType) String() string {
	return limitType.string()
}

func (limitType *LimitType) UnmarshalText(text []byte) error {
	return limitType.unmarshalText(text)
}

func (protocol Protocol) MarshalText() ([]byte, error) {
	return protocol.marshalText()
}

func (protocol Protocol) String() string {
	return protocol.string()
}

func (protocol *Protocol) UnmarshalText(text []byte) error {
	return protocol.unmarshalText(text)
}

// MethodLimit describes a token-bucket rate limit. A non-positive
// RequestsPerSecond disables the limit. Burst must be positive whenever
// RequestsPerSecond is positive.
type MethodLimit struct {
	RequestsPerSecond float64
	Burst             int
}

// PerUserPerMethodLimits configures the per-user-per-method tier. Default
// applies to every (user, method) pair that does not appear in Overrides.
type PerUserPerMethodLimits struct {
	Default   MethodLimit
	Overrides map[string]MethodLimit `json:",omitempty"`
}

// Limits is the top-level rate-limiting configuration consumed by New.
// A zero-valued tier is treated as unlimited.
type Limits struct {
	Global           MethodLimit
	PerMethod        map[string]MethodLimit `json:",omitempty"`
	PerUserPerMethod PerUserPerMethodLimits
}

// Options configures non-policy aspects of a Limiter.
type Options struct {
	Logger            log.DebugLogger
	MetricsSubDirname string // Under "ratelimit". If empty: no metrics.
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
	limitType LimitType
	protocol  Protocol
}

// New returns an error if a tier has a positive RequestsPerSecond but a
// non-positive Burst, or if the metrics directory cannot be registered.
func New(limits Limits, opts Options) (*Limiter, error) {
	return newLimiter(limits, opts)
}

// Allow tests a request against the tiers in order, returning nil if it is
// admitted. On denial it returns a *errors.ResourceExhaustedError naming the
// method and the LimitType which denied it, which callers may forward
// directly: it carries a GrpcCode() of codes.ResourceExhausted.
//
// An empty username, or a true bypassPerUser (such as a caller with method
// powers), skips the per-user-per-method tier; the others still apply.
func (l *Limiter) Allow(method, username string, protocol Protocol,
	bypassPerUser bool) error {
	return l.allow(method, username, protocol, bypassPerUser)
}

// DeniedCount returns the denials recorded for a (method, limitType,
// protocol) triple. It is safe to call concurrently.
func (l *Limiter) DeniedCount(method string, limitType LimitType,
	protocol Protocol) uint64 {
	return l.deniedCount(method, limitType, protocol)
}
