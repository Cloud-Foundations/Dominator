package smallstack

import (
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/slavedriver"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

type closeRequestMessage struct {
	errorChannel chan<- error
}

type SlaveTrader struct {
	closeChannel      chan<- closeRequestMessage
	hypervisorChannel <-chan *srpc.Client
	logger            log.DebugLogger
	options           SlaveTraderOptions
}

type SlaveTraderOptions struct {
	CreateRequest     hyper_proto.CreateVmRequest
	CreateTimeout     time.Duration // Default: 5 minutes.
	DestroyTimeout    time.Duration // Default: 1 minute.
	HypervisorAddress string        // Default: local Hypervisor.
}

func NewSlaveTrader(createRequest hyper_proto.CreateVmRequest,
	logger log.DebugLogger) (*SlaveTrader, error) {
	return newSlaveTrader(SlaveTraderOptions{CreateRequest: createRequest},
		logger)
}

func NewSlaveTraderWithOptions(options SlaveTraderOptions,
	logger log.DebugLogger) (*SlaveTrader, error) {
	return newSlaveTrader(options, logger)
}

func (trader *SlaveTrader) Close() error {
	return trader.close()
}

func (trader *SlaveTrader) CreateSlave() (slavedriver.SlaveInfo, error) {
	return trader.createSlave(nil)
}

func (trader *SlaveTrader) CreateSlaveWithAcknowledger(
	acknowledgeChannel <-chan chan<- error) (slavedriver.SlaveInfo, error) {
	return trader.createSlave(acknowledgeChannel)
}

func (trader *SlaveTrader) DestroySlave(identifier string) error {
	return trader.destroySlave(identifier)
}
