package dhcpd

import (
	"net"
	"sync"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/net/util"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

type DhcpServer struct {
	logger           log.DebugLogger
	myIPs            []net.IP
	networkBootImage []byte
	requestInterface string
	routeTable       map[string]*util.RouteEntry // Key: interface name.
	mutex            sync.RWMutex                // Protect everything below.
	ackChannels      map[string]chan struct{}    // Key: IPaddr.
	ipAddrToMacAddr  map[string]string           // Key: IPaddr, V: MACaddr.
	leases           map[string]leaseType        // Key: MACaddr.
	requestChannels  map[string]chan net.IP      // Key: MACaddr.
	subnets          []*subnetType
}

type leaseType struct {
	proto.Address
	Hostname  string
	doNetboot bool
	subnet    *subnetType
}

type subnetType struct {
	myIP net.IP
	proto.Subnet
}

func New(interfaceNames []string, logger log.DebugLogger) (*DhcpServer, error) {
	return newServer(interfaceNames, logger)
}

func (s *DhcpServer) AddLease(address proto.Address, hostname string) error {
	return s.addLease(address, false, hostname, nil)
}

func (s *DhcpServer) AddNetbootLease(address proto.Address,
	hostname string, subnet *proto.Subnet) error {
	return s.addLease(address, true, hostname, subnet)
}

func (s *DhcpServer) AddSubnet(subnet proto.Subnet) {
	s.addSubnet(subnet)
}

func (s *DhcpServer) MakeAcknowledgmentChannel(ipAddr net.IP) <-chan struct{} {
	return s.makeAcknowledgmentChannel(ipAddr)
}

func (s *DhcpServer) MakeRequestChannel(macAddr string) <-chan net.IP {
	return s.makeRequestChannel(macAddr)
}

func (s *DhcpServer) RemoveLease(ipAddr net.IP) {
	s.removeLease(ipAddr)
}

func (s *DhcpServer) RemoveSubnet(subnetId string) {
	s.removeSubnet(subnetId)
}

func (s *DhcpServer) SetNetworkBootImage(nbiName string) error {
	s.networkBootImage = []byte(nbiName)
	return nil
}
