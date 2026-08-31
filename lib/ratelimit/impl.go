package ratelimit

import (
	"fmt"

	"github.com/Cloud-Foundations/tricorder/go/tricorder"
	"github.com/Cloud-Foundations/tricorder/go/tricorder/units"
	"golang.org/x/time/rate"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
)

// Bounds the time l.mutex is held reclaiming buckets on the request path.
const maxBucketsSwept = 16

const (
	limitTypeUnknown = "UNKNOWN LimitType"
	protocolUnknown  = "UNKNOWN Protocol"
)

var (
	limitTypeToText = map[LimitType]string{
		LimitTypeGlobal:           "global",
		LimitTypePerMethod:        "per_method",
		LimitTypePerUserPerMethod: "per_user_per_method",
	}
	textToLimitType map[string]LimitType

	protocolToText = map[Protocol]string{
		ProtocolSRPC: "srpc",
		ProtocolGRPC: "grpc",
		ProtocolREST: "rest",
	}
	textToProtocol map[string]Protocol
)

func init() {
	textToLimitType = make(map[string]LimitType, len(limitTypeToText))
	for limitType, text := range limitTypeToText {
		textToLimitType[text] = limitType
	}
	textToProtocol = make(map[string]Protocol, len(protocolToText))
	for protocol, text := range protocolToText {
		textToProtocol[text] = protocol
	}
}

func (limitType LimitType) marshalText() ([]byte, error) {
	if text := limitType.string(); text == limitTypeUnknown {
		return nil, fmt.Errorf("invalid LimitType: %d", limitType)
	} else {
		return []byte(text), nil
	}
}

func (limitType LimitType) string() string {
	if str, ok := limitTypeToText[limitType]; !ok {
		return limitTypeUnknown
	} else {
		return str
	}
}

func (limitType *LimitType) unmarshalText(text []byte) error {
	txt := string(text)
	if val, ok := textToLimitType[txt]; ok {
		*limitType = val
		return nil
	} else {
		return fmt.Errorf("unknown LimitType: %s", txt)
	}
}

func (protocol Protocol) marshalText() ([]byte, error) {
	if text := protocol.string(); text == protocolUnknown {
		return nil, fmt.Errorf("invalid Protocol: %d", protocol)
	} else {
		return []byte(text), nil
	}
}

func (protocol Protocol) string() string {
	if str, ok := protocolToText[protocol]; !ok {
		return protocolUnknown
	} else {
		return str
	}
}

func (protocol *Protocol) unmarshalText(text []byte) error {
	txt := string(text)
	if val, ok := textToProtocol[txt]; ok {
		*protocol = val
		return nil
	} else {
		return fmt.Errorf("unknown Protocol: %s", txt)
	}
}

func newLimiter(limits Limits, opts Options) (*Limiter, error) {
	global, err := newBucket(limits.Global, "Global")
	if err != nil {
		return nil, err
	}
	perMethod := make(map[string]*rate.Limiter, len(limits.PerMethod))
	for method, ml := range limits.PerMethod {
		bucket, err := newBucket(ml, "PerMethod."+method)
		if err != nil {
			return nil, err
		}
		if bucket != nil {
			perMethod[method] = bucket
		}
	}
	if _, err := newBucket(limits.PerUserPerMethod.Default,
		"PerUserPerMethod.Default"); err != nil {
		return nil, err
	}
	for method, ml := range limits.PerUserPerMethod.Overrides {
		if _, err := newBucket(ml,
			"PerUserPerMethod.Overrides."+method); err != nil {
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
	if opts.MetricsSubDirname != "" {
		dirname := "ratelimit/" + opts.MetricsSubDirname
		dir, err := tricorder.RegisterDirectory(dirname)
		if err != nil {
			return nil, fmt.Errorf("registering metrics directory %q: %s",
				dirname, err)
		}
		limiter.metricsDir = dir
	}
	return limiter, nil
}

// newBucket returns nil if the limit is disabled.
func newBucket(ml MethodLimit, label string) (*rate.Limiter, error) {
	if ml.RequestsPerSecond <= 0 {
		return nil, nil
	}
	if ml.Burst <= 0 {
		return nil, fmt.Errorf(
			"%s: Burst must be positive when RequestsPerSecond is positive",
			label)
	}
	return rate.NewLimiter(rate.Limit(ml.RequestsPerSecond), ml.Burst), nil
}

func (l *Limiter) allow(method, username string, protocol Protocol,
	bypassPerUser bool) error {
	if l.global != nil && !l.global.Allow() {
		l.recordDenial(method, LimitTypeGlobal, protocol, username)
		return errors.NewResourceExhaustedError(method,
			LimitTypeGlobal.String())
	}
	if pm := l.perMethod[method]; pm != nil && !pm.Allow() {
		l.recordDenial(method, LimitTypePerMethod, protocol, username)
		return errors.NewResourceExhaustedError(method,
			LimitTypePerMethod.String())
	}
	// An empty username means an unauthenticated caller (srpc permits those
	// only for UnauthenticatedMethods): no identity to bucket on.
	if bypassPerUser || username == "" {
		return nil
	}
	bucket := l.getOrCreatePerUserBucket(method, username)
	if bucket != nil && !bucket.Allow() {
		l.recordDenial(method, LimitTypePerUserPerMethod, protocol, username)
		return errors.NewResourceExhaustedError(method,
			LimitTypePerUserPerMethod.String())
	}
	return nil
}

// getOrCreatePerUserBucket returns nil if the tier is disabled for the method.
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
	l.reclaimBucketsLocked()
	bucket := rate.NewLimiter(rate.Limit(cfg.RequestsPerSecond), cfg.Burst)
	l.perUser[key] = bucket
	return bucket
}

// reclaimBucketsLocked discards replenished buckets, bounding the map. A
// drained one is kept: discarding it would hand its owner a fresh burst.
func (l *Limiter) reclaimBucketsLocked() {
	examined := 0
	for key, bucket := range l.perUser {
		if examined >= maxBucketsSwept {
			return
		}
		examined++
		if bucket.Tokens() >= float64(bucket.Burst()) {
			delete(l.perUser, key)
		}
	}
}

func (l *Limiter) recordDenial(method string, limitType LimitType,
	protocol Protocol, username string) {
	key := denialKey{method: method, limitType: limitType, protocol: protocol}
	var metricPath string
	var registrationError error
	l.countersMu.Lock()
	counter, ok := l.counters[key]
	if !ok {
		counter = new(uint64)
		if l.metricsDir != nil {
			metricPath = method + "/" + limitType.String() + "/" +
				protocol.String()
			registrationError = l.metricsDir.RegisterMetric(metricPath,
				counter, units.None, "rate-limit denials")
		}
		l.counters[key] = counter
	}
	*counter++
	l.countersMu.Unlock()
	// Best effort: the counter is still readable via DeniedCount, and a
	// limiter must not bring down the request path it guards.
	if registrationError != nil && l.logger != nil {
		l.logger.Printf("ratelimit: registering metric %q: %s\n",
			metricPath, registrationError)
	}
	if l.logger != nil {
		l.logger.Debugf(0,
			"rate limit denied: user=%q method=%q limit=%s protocol=%s\n",
			username, method, limitType, protocol)
	}
}

func (l *Limiter) deniedCount(method string, limitType LimitType,
	protocol Protocol) uint64 {
	key := denialKey{method: method, limitType: limitType, protocol: protocol}
	l.countersMu.Lock()
	defer l.countersMu.Unlock()
	if counter, ok := l.counters[key]; ok {
		return *counter
	}
	return 0
}
