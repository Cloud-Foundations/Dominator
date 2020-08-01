package dhcpd

import (
	"errors"
	"net"
	"strings"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/prefixlogger"
	libnet "github.com/Cloud-Foundations/Dominator/lib/net"
	"github.com/Cloud-Foundations/Dominator/lib/net/util"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
	dhcp "github.com/krolaw/dhcp4"
	"golang.org/x/net/ipv4"
)

const sysClassNet = "/sys/class/net"
const leaseTime = time.Hour * 48

type serveIfConn struct {
	ifIndices        map[int]string
	conn             *ipv4.PacketConn
	cm               *ipv4.ControlMessage
	requestInterface *string
}

func listMyIPs() ([]net.IP, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	ipMap := make(map[string]net.IP)
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if iface.Flags&net.FlagBroadcast == 0 {
			continue
		}
		interfaceAddrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}
		for _, addr := range interfaceAddrs {
			IP, _, err := net.ParseCIDR(addr.String())
			if err != nil {
				return nil, err
			}
			if IP = IP.To4(); IP == nil {
				continue
			}
			ipMap[IP.String()] = IP
		}
	}
	var IPs []net.IP
	for _, IP := range ipMap {
		IPs = append(IPs, IP)
	}
	return IPs, nil
}

func newServer(interfaceNames []string, logger log.DebugLogger) (
	*DhcpServer, error) {
	logger = prefixlogger.New("dhcpd: ", logger)
	dhcpServer := &DhcpServer{
		logger:          logger,
		ackChannels:     make(map[string]chan struct{}),
		ipAddrToMacAddr: make(map[string]string),
		leases:          make(map[string]leaseType),
		requestChannels: make(map[string]chan net.IP),
		routeTable:      make(map[string]*util.RouteEntry),
	}
	if myIPs, err := listMyIPs(); err != nil {
		return nil, err
	} else {
		if len(myIPs) < 1 {
			return nil, errors.New("no IP addresses found")
		}
		dhcpServer.myIPs = myIPs
	}
	routeTable, err := util.GetRouteTable()
	if err != nil {
		return nil, err
	}
	for _, routeEntry := range routeTable.RouteEntries {
		if len(routeEntry.GatewayAddr) < 1 ||
			routeEntry.GatewayAddr.Equal(net.IPv4zero) {
			dhcpServer.routeTable[routeEntry.InterfaceName] = routeEntry
		}
	}
	if len(interfaceNames) < 1 {
		logger.Debugln(0, "Starting server on all broadcast interfaces")
		interfaces, _, err := libnet.ListBroadcastInterfaces(
			libnet.InterfaceTypeEtherNet|
				libnet.InterfaceTypeBridge|
				libnet.InterfaceTypeVlan,
			logger)
		if err != nil {
			return nil, err
		} else {
			for _, iface := range interfaces {
				interfaceNames = append(interfaceNames, iface.Name)
			}
		}
	} else {
		logger.Debugln(0, "Starting server on interfaces: "+
			strings.Join(interfaceNames, ","))
	}
	serveConn := &serveIfConn{
		ifIndices:        make(map[int]string, len(interfaceNames)),
		requestInterface: &dhcpServer.requestInterface,
	}
	for _, interfaceName := range interfaceNames {
		if iface, err := net.InterfaceByName(interfaceName); err != nil {
			return nil, err
		} else {
			serveConn.ifIndices[iface.Index] = iface.Name
		}
	}
	listener, err := net.ListenPacket("udp4", ":67")
	if err != nil {
		return nil, err
	}
	pktConn := ipv4.NewPacketConn(listener)
	if err := pktConn.SetControlMessage(ipv4.FlagInterface, true); err != nil {
		listener.Close()
		return nil, err
	}
	serveConn.conn = pktConn
	go func() {
		if err := dhcp.Serve(serveConn, dhcpServer); err != nil {
			logger.Println(err)
		}
	}()
	return dhcpServer, nil
}

func (s *DhcpServer) acknowledgeLease(ipAddr net.IP) {
	ipStr := ipAddr.String()
	s.mutex.Lock()
	ackChan, ok := s.ackChannels[ipStr]
	delete(s.ackChannels, ipStr)
	s.mutex.Unlock()
	if ok {
		ackChan <- struct{}{}
		close(ackChan)
	}
}

func (s *DhcpServer) addLease(address proto.Address, doNetboot bool,
	hostname string, protoSubnet *proto.Subnet) error {
	address.Shrink()
	if len(address.IpAddress) < 1 {
		return errors.New("no IP address")
	}
	ipAddr := address.IpAddress.String()
	var subnet *subnetType
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if protoSubnet == nil {
		if subnet = s.findMatchingSubnet(address.IpAddress); subnet == nil {
			return errors.New("no subnet found for " + ipAddr)
		}
	} else {
		subnet = s.makeSubnet(protoSubnet)
	}
	if doNetboot {
		if len(s.networkBootImage) < 1 {
			return errors.New("no Network Boot Image name configured")
		}
		if _, ok := s.leases[address.MacAddress]; ok {
			return errors.New("already have lease for: " + address.MacAddress)
		}
	}
	s.ipAddrToMacAddr[ipAddr] = address.MacAddress
	s.leases[address.MacAddress] = leaseType{
		address, hostname, doNetboot, subnet}
	return nil
}

func (s *DhcpServer) addSubnet(protoSubnet proto.Subnet) {
	subnet := s.makeSubnet(&protoSubnet)
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.subnets = append(s.subnets, subnet)
}

func (s *DhcpServer) checkRouteOnInterface(addr net.IP,
	interfaceName string) bool {
	if route, ok := s.routeTable[interfaceName]; !ok {
		return true
	} else if route.Flags&util.RouteFlagUp == 0 {
		return true
	} else if addr.Mask(route.Mask).Equal(route.BaseAddr) {
		return true
	}
	return false
}

func (s *DhcpServer) findLease(macAddr string) (*leaseType, *subnetType) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	if lease, ok := s.leases[macAddr]; !ok {
		return nil, nil
	} else if lease.subnet != nil {
		return &lease, lease.subnet
	} else {
		return &lease, s.findMatchingSubnet(lease.IpAddress)
	}
}

// This must be called with the lock held.
func (s *DhcpServer) findMatchingSubnet(ipAddr net.IP) *subnetType {
	for _, subnet := range s.subnets {
		subnetMask := net.IPMask(subnet.IpMask)
		subnetAddr := subnet.IpGateway.Mask(subnetMask)
		if ipAddr.Mask(subnetMask).Equal(subnetAddr) {
			return subnet
		}
	}
	return nil
}

func (s *DhcpServer) makeAcknowledgmentChannel(ipAddr net.IP) <-chan struct{} {
	ipStr := ipAddr.String()
	newChan := make(chan struct{}, 1)
	s.mutex.Lock()
	oldChan, ok := s.ackChannels[ipStr]
	s.ackChannels[ipStr] = newChan
	s.mutex.Unlock()
	if ok {
		close(oldChan)
	}
	return newChan
}

func (s *DhcpServer) makeOptions(subnet *subnetType,
	lease *leaseType) dhcp.Options {
	dnsServers := make([]byte, 0)
	for _, dnsServer := range subnet.DomainNameServers {
		dnsServers = append(dnsServers, dnsServer...)
	}
	leaseOptions := dhcp.Options{
		dhcp.OptionSubnetMask:       subnet.IpMask,
		dhcp.OptionRouter:           subnet.IpGateway,
		dhcp.OptionDomainNameServer: dnsServers,
	}
	if subnet.DomainName != "" {
		leaseOptions[dhcp.OptionDomainName] = []byte(subnet.DomainName)
	}
	if lease.Hostname != "" {
		leaseOptions[dhcp.OptionHostName] = []byte(lease.Hostname)
	}
	if lease.doNetboot {
		leaseOptions[dhcp.OptionBootFileName] = s.networkBootImage
	}
	return leaseOptions
}

func (s *DhcpServer) makeRequestChannel(macAddr string) <-chan net.IP {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if oldChan, ok := s.requestChannels[macAddr]; ok {
		return oldChan
	}
	newChan := make(chan net.IP, 16)
	s.requestChannels[macAddr] = newChan
	return newChan
}

func (s *DhcpServer) makeSubnet(protoSubnet *proto.Subnet) *subnetType {
	subnet := &subnetType{Subnet: *protoSubnet, myIP: s.myIPs[0]}
	for _, ip := range s.myIPs {
		subnetMask := net.IPMask(subnet.IpMask)
		subnetAddr := subnet.IpGateway.Mask(subnetMask)
		if ip.Mask(subnetMask).Equal(subnetAddr) {
			subnet.myIP = ip
			break
		}
	}
	return subnet
}

func (s *DhcpServer) notifyRequest(address proto.Address) {
	s.mutex.RLock()
	requestChan, ok := s.requestChannels[address.MacAddress]
	s.mutex.RUnlock()
	if ok {
		select {
		case requestChan <- address.IpAddress:
		default:
		}
	}
}

func (s *DhcpServer) removeLease(ipAddr net.IP) {
	if len(ipAddr) < 1 {
		return
	}
	ipStr := ipAddr.String()
	s.mutex.Lock()
	delete(s.leases, s.ipAddrToMacAddr[ipStr])
	delete(s.ipAddrToMacAddr, ipStr)
	ackChan, ok := s.ackChannels[ipStr]
	delete(s.ackChannels, ipStr)
	s.mutex.Unlock()
	if ok {
		close(ackChan)
	}
}

func (s *DhcpServer) removeSubnet(subnetId string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	subnets := make([]*subnetType, 0, len(s.subnets)-1)
	for _, subnet := range s.subnets {
		if subnet.Id != subnetId {
			subnets = append(subnets, subnet)
		}
	}
	s.subnets = subnets
}

func (s *DhcpServer) ServeDHCP(req dhcp.Packet, msgType dhcp.MessageType,
	options dhcp.Options) dhcp.Packet {
	switch msgType {
	case dhcp.Discover:
		macAddr := req.CHAddr().String()
		s.logger.Debugf(1, "Discover from: %s on: %s\n",
			macAddr, s.requestInterface)
		lease, subnet := s.findLease(macAddr)
		if lease == nil {
			return nil
		}
		if subnet == nil {
			s.logger.Printf("No subnet found for %s\n", lease.IpAddress)
			return nil
		}
		if !s.checkRouteOnInterface(lease.IpAddress, s.requestInterface) {
			s.logger.Printf(
				"suppressing offer: %s for: %s, wrong interface: %s\n",
				lease.IpAddress, macAddr, s.requestInterface)
			return nil
		}
		s.logger.Debugf(0, "Offer: %s for: %s, server: %s\n",
			lease.IpAddress, macAddr, subnet.myIP)
		leaseOptions := s.makeOptions(subnet, lease)
		packet := dhcp.ReplyPacket(req, dhcp.Offer, subnet.myIP,
			lease.IpAddress, leaseTime,
			leaseOptions.SelectOrderOrAll(
				options[dhcp.OptionParameterRequestList]))
		packet.SetSIAddr(subnet.myIP)
		return packet
	case dhcp.Request:
		reqIP := net.IP(options[dhcp.OptionRequestedIPAddress])
		if reqIP == nil {
			s.logger.Debugln(0, "Request did not request an IP")
			reqIP = net.IP(req.CIAddr())
		}
		reqIP = util.ShrinkIP(reqIP)
		macAddr := req.CHAddr().String()
		s.notifyRequest(proto.Address{reqIP, macAddr})
		server, ok := options[dhcp.OptionServerIdentifier]
		if ok {
			serverIP := net.IP(server)
			if !serverIP.IsUnspecified() {
				isMe := false
				for _, ip := range s.myIPs {
					if serverIP.Equal(ip) {
						isMe = true
						break
					}
				}
				if !isMe {
					s.logger.Debugf(0,
						"Request for: %s from: %s to: %s is not me\n",
						reqIP, macAddr, serverIP)
					return nil // Message not for this DHCP server.
				}
			}
		}
		s.logger.Debugf(0, "Request for: %s from: %s on: %s\n",
			reqIP, macAddr, s.requestInterface)
		lease, subnet := s.findLease(macAddr)
		if lease == nil {
			s.logger.Printf("No lease found for %s\n", macAddr)
			return nil
		}
		if subnet == nil {
			s.logger.Printf("No subnet found for %s\n", lease.IpAddress)
			return nil
		}
		if reqIP.Equal(lease.IpAddress) &&
			s.checkRouteOnInterface(lease.IpAddress, s.requestInterface) {
			leaseOptions := s.makeOptions(subnet, lease)
			s.logger.Debugf(0, "ACK for: %s to: %s, server: %s\n",
				reqIP, macAddr, subnet.myIP)
			s.acknowledgeLease(lease.IpAddress)
			packet := dhcp.ReplyPacket(req, dhcp.ACK, subnet.myIP, reqIP,
				leaseTime, leaseOptions.SelectOrderOrAll(
					options[dhcp.OptionParameterRequestList]))
			packet.SetSIAddr(subnet.myIP)
			return packet
		} else {
			s.logger.Debugf(0, "NAK for: %s to: %s\n", reqIP, macAddr)
			return dhcp.ReplyPacket(req, dhcp.NAK, subnet.myIP, nil, 0, nil)
		}
	default:
		s.logger.Debugf(0, "Unsupported message type: %s on: %s\n",
			msgType, s.requestInterface)
	}
	return nil
}

func (s *serveIfConn) ReadFrom(b []byte) (n int, addr net.Addr, err error) {
	for {
		n, s.cm, addr, err = s.conn.ReadFrom(b)
		if err != nil || s.cm == nil {
			*s.requestInterface = "UNKNOWN"
			break
		}
		if name, ok := s.ifIndices[s.cm.IfIndex]; ok {
			*s.requestInterface = name
			break
		}
	}
	return
}

func (s *serveIfConn) WriteTo(b []byte, addr net.Addr) (n int, err error) {
	s.cm.Src = nil
	return s.conn.WriteTo(b, s.cm, addr)
}
