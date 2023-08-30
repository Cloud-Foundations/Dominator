package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/dom/herd"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/srpc/serverutil"
)

type rpcType struct {
	herd   *herd.Herd
	logger log.Logger
	*serverutil.PerUserMethodLimiter
}

func Setup(herd *herd.Herd, logger log.Logger) {
	rpcObj := &rpcType{
		herd:   herd,
		logger: logger,
		PerUserMethodLimiter: serverutil.NewPerUserMethodLimiter(
			map[string]uint{
				"ClearSafetyShutoff":    1,
				"ForceDisruptiveUpdate": 1,
				"GetInfoForSubs":        1,
				"ListSubs":              1,
			}),
	}
	srpc.RegisterNameWithOptions("Dominator", rpcObj,
		srpc.ReceiverOptions{
			PublicMethods: []string{
				"ClearSafetyShutoff",
				"ForceDisruptiveUpdate",
				"GetInfoForSubs",
				"ListSubs",
			}})
}
