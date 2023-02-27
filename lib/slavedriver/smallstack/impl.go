package smallstack

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/backoffdelay"
	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/slavedriver"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/tags"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

const (
	linklocalAddress = "169.254.169.254"
	metadataUrl      = "http://" + linklocalAddress + "/"
	identityPath     = "latest/dynamic/instance-identity/document"
)

type slaveTrader struct {
	closeChannel      <-chan closeRequestMessage
	hypervisor        *srpc.Client
	hypervisorChannel chan<- *srpc.Client
	logger            log.DebugLogger
	nextPing          time.Time
	options           SlaveTraderOptions
}

var (
	myVmInfo hyper_proto.VmInfo
)

func readVmInfo(vmInfo *hyper_proto.VmInfo) error {
	url := metadataUrl + identityPath
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("error getting: %s: %s", url, resp.Status)
	}
	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(vmInfo); err != nil {
		return fmt.Errorf("error decoding identity document: %s", err)
	}
	return nil
}

func newSlaveTrader(options SlaveTraderOptions,
	logger log.DebugLogger) (*SlaveTrader, error) {
	if options.HypervisorAddress == "" {
		options.HypervisorAddress = fmt.Sprintf("%s:%d",
			linklocalAddress, constants.HypervisorPortNumber)
	} else if !strings.Contains(options.HypervisorAddress, ":") {
		options.HypervisorAddress += fmt.Sprintf(":%d",
			constants.HypervisorPortNumber)
	}
	if err := readVmInfo(&myVmInfo); err != nil {
		return nil, err
	}
	if options.CreateRequest.Hostname == "" {
		options.CreateRequest.Hostname = myVmInfo.Hostname + "-slave"
	}
	if options.CreateRequest.ImageName == "" {
		options.CreateRequest.ImageName = myVmInfo.ImageName
	}
	if options.CreateRequest.MemoryInMiB < 1 {
		options.CreateRequest.MemoryInMiB = myVmInfo.MemoryInMiB
	}
	if options.CreateRequest.MilliCPUs < 1 {
		options.CreateRequest.MilliCPUs = myVmInfo.MilliCPUs
	}
	if options.CreateRequest.MinimumFreeBytes < 1 {
		options.CreateRequest.MinimumFreeBytes = 256 << 20
	}
	if options.CreateRequest.RoundupPower < 1 {
		options.CreateRequest.RoundupPower = 26
	}
	if options.CreateRequest.SubnetId == "" {
		options.CreateRequest.SubnetId = myVmInfo.SubnetId
	}
	if options.CreateRequest.Tags["Name"] == "" {
		options.CreateRequest.Tags = tags.Tags{
			"Name": options.CreateRequest.Hostname}
	}
	closeChannel := make(chan closeRequestMessage)
	hypervisorChannel := make(chan *srpc.Client)
	privateTrader := &slaveTrader{
		closeChannel:      closeChannel,
		hypervisorChannel: hypervisorChannel,
		logger:            logger,
		options:           options,
	}
	publicTrader := &SlaveTrader{
		closeChannel:      closeChannel,
		hypervisorChannel: hypervisorChannel,
		logger:            logger,
		options:           options,
	}
	go privateTrader.ultraVisor()
	return publicTrader, nil
}

func (trader *SlaveTrader) close() error {
	errorChannel := make(chan error)
	trader.closeChannel <- closeRequestMessage{errorChannel: errorChannel}
	close(trader.closeChannel)
	return <-errorChannel
}

func (trader *SlaveTrader) createSlave() (slavedriver.SlaveInfo, error) {
	if hyperClient, err := trader.getHypervisor(); err != nil {
		return slavedriver.SlaveInfo{}, err
	} else {
		var reply hyper_proto.CreateVmResponse
		err := client.CreateVm(hyperClient, trader.options.CreateRequest,
			&reply, trader.logger)
		if err != nil {
			return slavedriver.SlaveInfo{},
				fmt.Errorf("error creating VM: %s", err)
		}
		err = client.AcknowledgeVm(hyperClient, reply.IpAddress)
		if err != nil {
			client.DestroyVm(hyperClient, reply.IpAddress, nil)
			return slavedriver.SlaveInfo{},
				fmt.Errorf("error acknowledging VM: %s", err)
		}
		if reply.DhcpTimedOut {
			client.DestroyVm(hyperClient, reply.IpAddress, nil)
			return slavedriver.SlaveInfo{},
				fmt.Errorf("DHCP timeout for: %s", reply.IpAddress)
		}
		return slavedriver.SlaveInfo{
			Identifier: reply.IpAddress.String(),
			IpAddress:  reply.IpAddress,
		}, nil
	}
}

func (trader *SlaveTrader) destroySlave(identifier string) error {
	ipAddr := net.ParseIP(identifier)
	if len(ipAddr) < 1 {
		return fmt.Errorf("error parsing: %s", identifier)
	}
	if ip4 := ipAddr.To4(); ip4 != nil {
		ipAddr = ip4
	}
	if hyperClient, err := trader.getHypervisor(); err != nil {
		return err
	} else if err := client.DestroyVm(hyperClient, ipAddr, nil); err != nil {
		if !strings.Contains(err.Error(), "no VM with IP address") {
			return err
		}
	}
	return nil
}

func (trader *SlaveTrader) getHypervisor() (*srpc.Client, error) {
	timer := time.NewTimer(5 * time.Minute)
	select {
	case client := <-trader.hypervisorChannel:
		if !timer.Stop() {
			<-timer.C
		}
		return client, nil
	case <-timer.C:
		return nil, fmt.Errorf("timed out connecting to Hypervisor")
	}
}

func (trader *slaveTrader) getHypervisor() *srpc.Client {
	sleeper := backoffdelay.NewExponential(100*time.Millisecond, 10*time.Second,
		1)
	for ; ; sleeper.Sleep() {
		client, err := srpc.DialHTTP("tcp", trader.options.HypervisorAddress,
			time.Second*5)
		if err != nil {
			trader.logger.Printf("error connecting to Hypervisor: %s: %s\n",
				trader.options.HypervisorAddress, err)
			continue
		}
		return client
	}
}

func (trader *slaveTrader) ultraVisor() {
	for {
		if trader.hypervisor == nil {
			trader.hypervisor = trader.getHypervisor()
			trader.nextPing = time.Now().Add(5 * time.Second)
		}
		pingTimeout := time.Until(trader.nextPing)
		if pingTimeout < 0 {
			pingTimeout = 0
		}
		pingTimer := time.NewTimer(pingTimeout)
		select {
		case closeMessage := <-trader.closeChannel:
			closeMessage.errorChannel <- trader.hypervisor.Close()
			return
		case trader.hypervisorChannel <- trader.hypervisor:
		case <-pingTimer.C:
			if err := trader.hypervisor.Ping(); err != nil {
				trader.logger.Printf(
					"error pinging Hypervisor: %s, reconnecting: %s\n",
					trader.options.HypervisorAddress, err)
				trader.hypervisor.Close()
				trader.hypervisor = nil
			} else {
				trader.nextPing = time.Now().Add(5 * time.Second)
			}
		}
		pingTimer.Stop()
		select {
		case <-pingTimer.C:
		default:
		}
	}
}
