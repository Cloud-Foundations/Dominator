package serverutil

import (
	"github.com/Cloud-Foundations/Dominator/lib/concurrentlimit"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

func newPerUserMethodLimiter(
	perUserMethodLimits map[string]uint) *PerUserMethodLimiter {
	return &PerUserMethodLimiter{
		inner: concurrentlimit.NewLimiter(perUserMethodLimits),
	}
}

func (limiter *PerUserMethodLimiter) blockMethod(methodName string,
	authInfo *srpc.AuthInformation) (func(), error) {
	return limiter.inner.Acquire(methodName, authInfo.Username,
		authInfo.HaveMethodAccess)
}
