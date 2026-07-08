package ratelimit

import (
	"fmt"

	"github.com/Cloud-Foundations/tricorder/go/tricorder"
	"github.com/Cloud-Foundations/tricorder/go/tricorder/units"
	"golang.org/x/time/rate"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
)

func newLimiter(limits Limits, opts Options) (*Limiter, error) {
	global, err := newBucket(limits.Global, "global")
	if err != nil {
		return nil, err
	}
	perMethod := make(map[string]*rate.Limiter, len(limits.PerMethod))
	for method, ml := range limits.PerMethod {
		bucket, err := newBucket(ml, "per_method."+method)
		if err != nil {
			return nil, err
		}
		if bucket != nil {
			perMethod[method] = bucket
		}
	}
	if _, err := newBucket(limits.PerUserPerMethod.Default,
		"per_user_per_method.default"); err != nil {
		return nil, err
	}
	for method, ml := range limits.PerUserPerMethod.Overrides {
		if _, err := newBucket(ml,
			"per_user_per_method.overrides."+method); err != nil {
			return nil, err
		}
	}
	limiter := &Limiter{
		logger:     opts.Logger,
		global:     global,
		perMethod:  perMethod,
		perUserCfg: limits.PerUserPerMethod,
		perUser:    make(map[userMethodType]*rate.Limiter),
		counters:   make(map[denialKey]*uint64),
	}
	if opts.MetricsDirectoryName != "" {
		dir, err := tricorder.RegisterDirectory(opts.MetricsDirectoryName)
		if err != nil {
			return nil, fmt.Errorf("registering metrics directory %q: %s",
				opts.MetricsDirectoryName, err)
		}
		limiter.metricsDir = dir
	}
	return limiter, nil
}

// newBucket returns a *rate.Limiter for the given configuration, or nil if the
// limit is disabled (RequestsPerSecond <= 0). It returns an error if
// RequestsPerSecond is positive but Burst is non-positive.
func newBucket(ml MethodLimit, label string) (*rate.Limiter, error) {
	if ml.RequestsPerSecond <= 0 {
		return nil, nil
	}
	if ml.Burst <= 0 {
		return nil, fmt.Errorf(
			"%s: burst must be positive when requests_per_second is positive",
			label)
	}
	return rate.NewLimiter(rate.Limit(ml.RequestsPerSecond), ml.Burst), nil
}

func (l *Limiter) allow(method, username string, protocol Protocol,
	bypassPerUser bool) error {
	if l.global != nil && !l.global.Allow() {
		l.recordDenial(method, LimitTypeGlobal, protocol, username)
		return errors.NewResourceExhaustedError(method, LimitTypeGlobal)
	}
	if pm := l.perMethod[method]; pm != nil && !pm.Allow() {
		l.recordDenial(method, LimitTypePerMethod, protocol, username)
		return errors.NewResourceExhaustedError(method, LimitTypePerMethod)
	}
	// Empty username means an unauthenticated caller reached a public or
	// unauthenticated method; the per-user-per-method tier has no identity
	// to bucket on, so we skip it. Global and per-method tiers still apply
	// above. If unauthenticated abuse becomes a concern, protocol adapters
	// could synthesise an identity from the peer address before calling
	// Allow, bucketing anonymous traffic without changing this contract.
	if !bypassPerUser && username != "" {
		if bucket := l.getOrCreatePerUserBucket(method, username); bucket != nil {
			if !bucket.Allow() {
				l.recordDenial(method, LimitTypePerUserPerMethod, protocol,
					username)
				return errors.NewResourceExhaustedError(method,
					LimitTypePerUserPerMethod)
			}
		}
	}
	return nil
}

// getOrCreatePerUserBucket returns the per-(user, method) token bucket,
// creating it lazily on first use. It returns nil if the per-user-per-method
// tier is disabled for the method (no override and a non-positive default).
func (l *Limiter) getOrCreatePerUserBucket(method,
	username string) *rate.Limiter {
	cfg, ok := l.perUserCfg.Overrides[method]
	if !ok {
		cfg = l.perUserCfg.Default
	}
	if cfg.RequestsPerSecond <= 0 {
		return nil
	}
	key := userMethodType{method: method, username: username}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if bucket, ok := l.perUser[key]; ok {
		return bucket
	}
	bucket := rate.NewLimiter(rate.Limit(cfg.RequestsPerSecond), cfg.Burst)
	l.perUser[key] = bucket
	return bucket
}

func (l *Limiter) recordDenial(method, limitType string, protocol Protocol,
	username string) {
	key := denialKey{method: method, limitType: limitType, protocol: protocol}
	l.countersMu.Lock()
	counter, ok := l.counters[key]
	if !ok {
		counter = new(uint64)
		if l.metricsDir != nil {
			path := method + "/" + limitType + "/" + string(protocol)
			if err := l.metricsDir.RegisterMetric(path, counter, units.None,
				"rate-limit denials"); err != nil {
				l.countersMu.Unlock()
				panic(err)
			}
		}
		l.counters[key] = counter
	}
	*counter++
	l.countersMu.Unlock()
	if l.logger != nil {
		l.logger.Debugf(0,
			"rate limit denied: user=%q method=%q limit=%s protocol=%s\n",
			username, method, limitType, protocol)
	}
}

func (l *Limiter) deniedCount(method, limitType string,
	protocol Protocol) uint64 {
	key := denialKey{method: method, limitType: limitType, protocol: protocol}
	l.countersMu.Lock()
	defer l.countersMu.Unlock()
	if counter, ok := l.counters[key]; ok {
		return *counter
	}
	return 0
}
