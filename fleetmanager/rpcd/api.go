package rpcd

import (
	"io"

	"github.com/Cloud-Foundations/Dominator/fleetmanager/hypervisors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/srpc/serverutil"
)

type Config struct {
}

type Params struct {
	HypervisorsManager *hypervisors.Manager
	Logger             log.DebugLogger
}

type srpcType struct {
	hypervisorsManager *hypervisors.Manager
	logger             log.DebugLogger
	*serverutil.PerUserMethodLimiter
}

type htmlWriter srpcType

func (hw *htmlWriter) WriteHtml(writer io.Writer) {
	hw.writeHtml(writer)
}

func Setup(config Config, params Params) (
	*htmlWriter, error) {
	srpcObj := &srpcType{
		hypervisorsManager: params.HypervisorsManager,
		logger:             params.Logger,
		PerUserMethodLimiter: serverutil.NewPerUserMethodLimiter(
			map[string]uint{
				"GetMachineInfo": 1,
				"GetUpdates":     1,
			}),
	}
	srpc.RegisterNameWithOptions("FleetManager", srpcObj,
		srpc.ReceiverOptions{
			PublicMethods: []string{
				"ChangeMachineTags",
				"GetHypervisorForVM",
				"GetHypervisorsInLocation",
				"GetIpInfo",
				"GetMachineInfo",
				"GetUpdates",
				"ListHypervisorLocations",
				"ListHypervisorsInLocation",
				"ListVMsInLocation",
				"PowerOnMachine",
			}})
	return (*htmlWriter)(srpcObj), nil
}
