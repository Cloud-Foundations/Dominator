package serverutil

import (
	"github.com/Cloud-Foundations/Dominator/lib/concurrentlimit"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

// PerUserMethodLimiter is an srpc.MethodBlocker adapter over
// *concurrentlimit.Limiter. It caps the number of simultaneously in-flight
// calls per (user, method) pair for an SRPC server. Callers with elevated
// method-power access (authInfo.HaveMethodAccess) bypass the limiter; the
// same *concurrentlimit.Limiter may be reused directly by future gRPC and
// REST adapters.
type PerUserMethodLimiter struct {
	inner *concurrentlimit.Limiter
}

func NewPerUserMethodLimiter(
	perUserMethodLimits map[string]uint) *PerUserMethodLimiter {
	return newPerUserMethodLimiter(perUserMethodLimits)
}

func (limiter *PerUserMethodLimiter) BlockMethod(methodName string,
	authInfo *srpc.AuthInformation) (func(), error) {
	return limiter.blockMethod(methodName, authInfo)
}
