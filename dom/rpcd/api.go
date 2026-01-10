package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/dom/herd"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/srpc/serverutil"
)

type Config struct {
	AllowRootAuthentication bool
}

type Params struct {
	Herd   *herd.Herd
	Logger log.DebugLogger
}

type rpcType struct {
	herd   *herd.Herd
	logger log.Logger
	*serverutil.PerUserMethodLimiter
}

func Setup(config Config, params Params) {
	rpcObj := &rpcType{
		herd:   params.Herd,
		logger: params.Logger,
		PerUserMethodLimiter: serverutil.NewPerUserMethodLimiter(
			map[string]uint{
				"ClearSafetyShutoff":    1,
				"ForceDisruptiveUpdate": 1,
				"GetInfoForSubs":        1,
				"ListSubs":              1,
			}),
	}
	publicMethods := []string{
		"ClearSafetyShutoff",
		"FastUpdate",
		"ForceDisruptiveUpdate",
		"GetInfoForSubs",
		"ListSubs",
	}
	var unauthenticatedMethods []string
	if config.AllowRootAuthentication {
		unauthenticatedMethods = append(unauthenticatedMethods, "FastUpdate")
	}
	srpc.RegisterNameWithOptions("Dominator", rpcObj,
		srpc.ReceiverOptions{
			PublicMethods:          publicMethods,
			UnauthenticatedMethods: unauthenticatedMethods,
		},
	)
}
