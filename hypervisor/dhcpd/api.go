package dhcpd

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/net/util"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

type DhcpServer struct {
	dynamicLeasesFile string
	logger            log.DebugLogger
	cleanupTrigger    chan<- struct{}
	interfaceIPs      map[string][]net.IP // Key: interface name.
	myIPs             []net.IP
	networkBootImage  string
	requestInterface  string
	routeTable        map[string]*util.RouteEntry // Key: interface name.
	mutex             sync.RWMutex                // Protect everything below.
	ackChannels       map[string]chan struct{}    // Key: IPaddr.
	dynamicLeases     map[string]*leaseType       // Key: MACaddr.
	interfaceSubnets  map[string][]*subnetType    // Key: interface name.
	ipAddrToMacAddr   map[string]string           // Key: IPaddr, V: MACaddr.
	packetWatchers    map[<-chan proto.WatchDhcpResponse]chan<- proto.WatchDhcpResponse
	requestChannels   map[string]chan net.IP // Key: MACaddr.
	staticLeases      map[string]leaseType   // Key: MACaddr.
	subnets           []*subnetType
}

type leaseType struct {
	proto.Address
	clientHostname string
	expires        time.Time
	hostname       string
	doNetboot      bool
	subnet         *subnetType
}

type subnetType struct {
	amGateway     bool
	myIP          net.IP
	nextDynamicIP net.IP
	proto.Subnet
}

func New(interfaceNames []string, dynamicLeasesFile string,
	logger log.DebugLogger) (*DhcpServer, error) {
	return newServer(interfaceNames, dynamicLeasesFile, logger)
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

func (s *DhcpServer) ClosePacketWatchChannel(
	channel <-chan proto.WatchDhcpResponse) {
	s.closePacketWatchChannel(channel)
}

func (s *DhcpServer) MakeAcknowledgmentChannel(ipAddr net.IP) <-chan struct{} {
	return s.makeAcknowledgmentChannel(ipAddr)
}

func (s *DhcpServer) MakePacketWatchChannel() <-chan proto.WatchDhcpResponse {
	return s.makePacketWatchChannel()
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
	s.networkBootImage = nbiName
	return nil
}

func (s *DhcpServer) WriteHtml(writer io.Writer) {
	s.writeHtml(writer)
}
