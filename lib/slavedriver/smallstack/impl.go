package smallstack

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Cloud-Foundations/Dominator/hypervisor/client"
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

var (
	myVmInfo hyper_proto.VmInfo
)

func newSlaveTrader(options SlaveTraderOptions,
	logger log.DebugLogger) (*SlaveTrader, error) {
	if options.HypervisorAddress == "" {
		options.HypervisorAddress = fmt.Sprintf("%s:%d",
			linklocalAddress, constants.HypervisorPortNumber)
	} else if !strings.Contains(options.HypervisorAddress, ":") {
		options.HypervisorAddress += fmt.Sprintf(":%d",
			constants.HypervisorPortNumber)
	}
	trader := &SlaveTrader{
		logger:  logger,
		options: options,
	}
	var err error
	trader.hypervisor, err = trader.getHypervisor()
	if err != nil {
		return nil, err
	}
	if err := readVmInfo(&myVmInfo); err != nil {
		trader.close()
		return nil, err
	}
	if trader.options.CreateRequest.Hostname == "" {
		trader.options.CreateRequest.Hostname = myVmInfo.Hostname + "-slave"
	}
	if trader.options.CreateRequest.ImageName == "" {
		trader.options.CreateRequest.ImageName = myVmInfo.ImageName
	}
	if trader.options.CreateRequest.MemoryInMiB < 1 {
		trader.options.CreateRequest.MemoryInMiB = myVmInfo.MemoryInMiB
	}
	if trader.options.CreateRequest.MilliCPUs < 1 {
		trader.options.CreateRequest.MilliCPUs = myVmInfo.MilliCPUs
	}
	if trader.options.CreateRequest.MinimumFreeBytes < 1 {
		trader.options.CreateRequest.MinimumFreeBytes = 256 << 20
	}
	if trader.options.CreateRequest.RoundupPower < 1 {
		trader.options.CreateRequest.RoundupPower = 26
	}
	if trader.options.CreateRequest.SubnetId == "" {
		trader.options.CreateRequest.SubnetId = myVmInfo.SubnetId
	}
	if trader.options.CreateRequest.Tags["Name"] == "" {
		trader.options.CreateRequest.Tags = tags.Tags{
			"Name": trader.options.CreateRequest.Hostname}
	}
	return trader, nil
}

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

func (trader *SlaveTrader) close() error {
	if trader.hypervisor == nil {
		return nil
	}
	err := trader.hypervisor.Close()
	trader.hypervisor = nil
	return err
}

func (trader *SlaveTrader) getHypervisor() (*srpc.Client, error) {
	trader.mutex.Lock()
	defer trader.mutex.Unlock()
	if hyperClient := trader.hypervisor; hyperClient != nil {
		if err := hyperClient.Ping(); err != nil {
			trader.logger.Printf("error pinging hypervisor, reconnecting: %s\n",
				err)
			hyperClient.Close()
			trader.hypervisor = nil
		} else {
			return trader.hypervisor, nil
		}
	}
	hyperClient, err := srpc.DialHTTP("tcp", trader.options.HypervisorAddress,
		time.Second*5)
	if err != nil {
		return nil, err
	}
	trader.hypervisor = hyperClient
	return hyperClient, nil
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
