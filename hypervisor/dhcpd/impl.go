package dhcpd

import (
	"errors"
	"net"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/html"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/prefixlogger"
	libnet "github.com/Cloud-Foundations/Dominator/lib/net"
	"github.com/Cloud-Foundations/Dominator/lib/net/util"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
	dhcp "github.com/krolaw/dhcp4"
	"golang.org/x/net/ipv4"
)

const dynamicLeaseTime = time.Hour * 4
const staticLeaseTime = time.Hour * 48

type dynamicLeaseType struct {
	ClientHostName string `json:",omitempty"`
	Expires        time.Time
	proto.Address
}

type serveIfConn struct {
	ifIndices        map[int]string
	conn             *ipv4.PacketConn
	cm               *ipv4.ControlMessage
	requestInterface *string
}

func listMyIPs() (map[string][]net.IP, []net.IP, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, nil, err
	}
	ifMap := make(map[string][]net.IP)
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
			return nil, nil, err
		}
		for _, addr := range interfaceAddrs {
			IP, _, err := net.ParseCIDR(addr.String())
			if err != nil {
				return nil, nil, err
			}
			if IP = IP.To4(); IP == nil {
				continue
			}
			ifMap[iface.Name] = append(ifMap[iface.Name], IP)
			ipMap[IP.String()] = IP
		}
	}
	var IPs []net.IP
	for _, IP := range ipMap {
		IPs = append(IPs, IP)
	}
	return ifMap, IPs, nil
}

func newServer(interfaceNames []string, dynamicLeasesFile string,
	logger log.DebugLogger) (
	*DhcpServer, error) {
	logger = prefixlogger.New("dhcpd: ", logger)
	cleanupTriggerChannel := make(chan struct{}, 1)
	dhcpServer := &DhcpServer{
		dynamicLeasesFile: dynamicLeasesFile,
		logger:            logger,
		cleanupTrigger:    cleanupTriggerChannel,
		ackChannels:       make(map[string]chan struct{}),
		interfaceSubnets:  make(map[string][]*subnetType),
		ipAddrToMacAddr:   make(map[string]string),
		staticLeases:      make(map[string]leaseType),
		requestChannels:   make(map[string]chan net.IP),
		routeTable:        make(map[string]*util.RouteEntry),
		dynamicLeases:     make(map[string]*leaseType),
	}
	if interfaceIPs, myIPs, err := listMyIPs(); err != nil {
		return nil, err
	} else {
		if len(myIPs) < 1 {
			return nil, errors.New("no IP addresses found")
		}
		dhcpServer.interfaceIPs = interfaceIPs
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
	if err := dhcpServer.readDynamicLeases(); err != nil {
		return nil, err
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
	go dhcpServer.cleanupDynamicLeasesLoop(cleanupTriggerChannel)
	html.HandleFunc("/showDhcpStatus", dhcpServer.showDhcpStatusHandler)
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
		if _, ok := s.staticLeases[address.MacAddress]; ok {
			return errors.New("already have lease for: " + address.MacAddress)
		}
	}
	if lease, ok := s.dynamicLeases[address.MacAddress]; ok {
		leaseIpAddr := lease.IpAddress.String()
		s.logger.Printf("discarding {%s %s}: static lease\n",
			leaseIpAddr, address.MacAddress)
		delete(s.ipAddrToMacAddr, leaseIpAddr)
		delete(s.dynamicLeases, address.MacAddress)
		if err := s.writeDynamicLeases(); err != nil {
			s.logger.Println(err)
		}
	}
	if lease, ok := s.staticLeases[address.MacAddress]; ok {
		leaseIpAddr := lease.IpAddress.String()
		s.logger.Printf("replacing {%s %s}\n", leaseIpAddr, address.MacAddress)
		delete(s.ipAddrToMacAddr, leaseIpAddr)
		delete(s.staticLeases, address.MacAddress)
	}
	if macAddr, ok := s.ipAddrToMacAddr[ipAddr]; ok {
		s.logger.Printf("replacing {%s %s}\n", ipAddr, macAddr)
		delete(s.ipAddrToMacAddr, ipAddr)
		delete(s.dynamicLeases, macAddr)
		delete(s.staticLeases, macAddr)
	}
	s.ipAddrToMacAddr[ipAddr] = address.MacAddress
	s.staticLeases[address.MacAddress] = leaseType{
		Address:   address,
		hostname:  hostname,
		doNetboot: doNetboot,
		subnet:    subnet,
	}
	return nil
}

func (s *DhcpServer) addSubnet(protoSubnet proto.Subnet) {
	subnet := s.makeSubnet(&protoSubnet)
	var ifaceName string
	for name, ips := range s.interfaceIPs {
		for _, ip := range ips {
			if protoSubnet.IpGateway.Equal(ip) {
				ifaceName = name
				s.logger.Printf("attaching subnet GW: %s to interface: %s\n",
					ip, name)
				break
			}
		}
		if ifaceName != "" {
			break
		}
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if ifaceName != "" {
		s.interfaceSubnets[ifaceName] = append(s.interfaceSubnets[ifaceName],
			subnet)
	}
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

func (s *DhcpServer) cleanupDynamicLeases() time.Duration {
	numExpired := 0
	waitTime := time.Hour
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for macAddr, lease := range s.dynamicLeases {
		expiresIn := time.Until(lease.expires)
		if expiresIn > 0 {
			if expiresIn < waitTime {
				waitTime = expiresIn
			}
		} else {
			delete(s.ipAddrToMacAddr, lease.Address.IpAddress.String())
			delete(s.dynamicLeases, macAddr)
			numExpired++
		}
	}
	if numExpired > 0 {
		s.logger.Debugf(0, "expired %d dynamic leases\n", numExpired)
		if err := s.writeDynamicLeases(); err != nil {
			s.logger.Println(err)
		}
	}
	return waitTime
}

func (s *DhcpServer) cleanupDynamicLeasesLoop(cleanupTrigger <-chan struct{}) {
	timer := time.NewTimer(time.Second)
	for {
		select {
		case <-cleanupTrigger:
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(s.cleanupDynamicLeases())
		case <-timer.C:
			timer.Reset(s.cleanupDynamicLeases())
		}
	}
}

// This must be called with the lock held.
func (s *DhcpServer) computeLeaseTime(lease *leaseType,
	offer bool) time.Duration {
	if lease.expires.IsZero() {
		return staticLeaseTime
	}
	if offer {
		return time.Minute
	}
	lease.expires = time.Now().Add(dynamicLeaseTime)
	select {
	case s.cleanupTrigger <- struct{}{}:
	default:
	}
	if err := s.writeDynamicLeases(); err != nil {
		s.logger.Println(err)
	}
	return dynamicLeaseTime >> 1
}

// This must be called with the lock held.
func (s *DhcpServer) findDynamicLease(macAddr, iface string) (
	*leaseType, *subnetType) {
	lease, ok := s.dynamicLeases[macAddr]
	if !ok {
		return nil, nil
	}
	if macAddr != lease.MacAddress {
		s.logger.Printf("discarding {%s %s}: bad MAC in lease for: %s\n",
			lease.IpAddress, lease.MacAddress, macAddr)
		delete(s.ipAddrToMacAddr, lease.IpAddress.String())
		delete(s.dynamicLeases, lease.MacAddress)
		delete(s.dynamicLeases, macAddr)
		return nil, nil
	}
	if lease.subnet == nil {
		if subnet := s.findMatchingSubnet(lease.IpAddress); subnet == nil {
			s.logger.Printf("discarding {%s %s}: no subnet\n",
				lease.IpAddress, lease.MacAddress)
			delete(s.ipAddrToMacAddr, lease.IpAddress.String())
			delete(s.dynamicLeases, lease.MacAddress)
			return nil, nil
		} else {
			lease.subnet = subnet
		}
	}
	if !lease.subnet.dynamicOK() {
		s.logger.Printf("discarding {%s %s}: no dynamic leases\n",
			lease.IpAddress, lease.MacAddress)
		delete(s.ipAddrToMacAddr, lease.IpAddress.String())
		delete(s.dynamicLeases, lease.MacAddress)
		return nil, nil
	}
	for _, subnet := range s.interfaceSubnets[iface] {
		if lease.subnet == subnet {
			return lease, subnet
		}
	}
	s.logger.Printf("discarding {%s %s}: no interface\n",
		lease.IpAddress, lease.MacAddress)
	delete(s.ipAddrToMacAddr, lease.IpAddress.String())
	delete(s.dynamicLeases, lease.MacAddress)
	return nil, nil
}

// This must be called with the lock held.
func (s *DhcpServer) findLease(macAddr, iface string, reqIP net.IP) (
	*leaseType, *subnetType) {
	if lease, subnet := s.findStaticLease(macAddr); lease != nil {
		return lease, subnet
	}
	if lease, subnet := s.findDynamicLease(macAddr, iface); lease != nil {
		return lease, subnet
	}
	for _, subnet := range s.interfaceSubnets[iface] {
		if lease := s.makeDynamicLease(macAddr, subnet, reqIP); lease != nil {
			return lease, subnet
		}
	}
	return nil, nil
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

// This must be called with the lock held.
func (s *DhcpServer) findStaticLease(macAddr string) (*leaseType, *subnetType) {
	if lease, ok := s.staticLeases[macAddr]; ok {
		if lease.subnet != nil {
			return &lease, lease.subnet
		} else {
			return &lease, s.findMatchingSubnet(lease.IpAddress)
		}
	}
	return nil, nil
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

// This must be called with the lock held.
func (s *DhcpServer) makeDynamicLease(macAddr string, subnet *subnetType,
	reqIP net.IP) *leaseType {
	if !subnet.dynamicOK() {
		return nil
	}
	stopIP := util.CopyIP(subnet.LastDynamicIP)
	util.IncrementIP(stopIP)
	if len(reqIP) == 4 {
		reqIpString := reqIP.String()
		if _, ok := s.ipAddrToMacAddr[reqIpString]; !ok {
			lowIP := util.CopyIP(subnet.FirstDynamicIP)
			util.DecrementIP(lowIP)
			if util.CompareIPs(lowIP, reqIP) && util.CompareIPs(reqIP, stopIP) {
				lease := leaseType{Address: proto.Address{
					IpAddress:  util.CopyIP(reqIP),
					MacAddress: macAddr,
				},
					expires: time.Now().Add(time.Second * 10),
					subnet:  subnet,
				}
				s.dynamicLeases[macAddr] = &lease
				s.ipAddrToMacAddr[reqIpString] = macAddr
				select {
				case s.cleanupTrigger <- struct{}{}:
				default:
				}
				return &lease
			}
		}
	}
	if len(subnet.nextDynamicIP) < 4 {
		subnet.nextDynamicIP = util.CopyIP(subnet.FirstDynamicIP)
	}
	initialIp := util.CopyIP(subnet.nextDynamicIP)
	for {
		testIp := util.CopyIP(subnet.nextDynamicIP)
		testIpString := testIp.String()
		util.IncrementIP(subnet.nextDynamicIP)
		if _, ok := s.ipAddrToMacAddr[testIpString]; !ok {
			lease := leaseType{Address: proto.Address{
				IpAddress:  testIp,
				MacAddress: macAddr,
			},
				expires: time.Now().Add(time.Second * 10),
				subnet:  subnet,
			}
			s.dynamicLeases[macAddr] = &lease
			s.ipAddrToMacAddr[testIpString] = macAddr
			select {
			case s.cleanupTrigger <- struct{}{}:
			default:
			}
			return &lease
		}
		if subnet.nextDynamicIP.Equal(stopIP) {
			copy(subnet.nextDynamicIP, subnet.FirstDynamicIP)
		}
		if initialIp.Equal(subnet.nextDynamicIP) {
			break
		}
	}
	return nil
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
	if lease.hostname != "" {
		leaseOptions[dhcp.OptionHostName] = []byte(lease.hostname)
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
		if ip.Equal(subnet.IpGateway) {
			subnet.amGateway = true
		}
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

// This must be called with the lock held.
func (s *DhcpServer) readDynamicLeases() error {
	var leases []dynamicLeaseType
	if err := json.ReadFromFile(s.dynamicLeasesFile, &leases); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	numExpiredLeases := 0
	numValidLeases := 0
	for _, lease := range leases {
		if time.Until(lease.Expires) > 0 {
			s.dynamicLeases[lease.Address.MacAddress] = &leaseType{
				Address:        lease.Address,
				clientHostname: lease.ClientHostName,
				expires:        lease.Expires,
			}
			s.ipAddrToMacAddr[lease.Address.IpAddress.String()] =
				lease.Address.MacAddress
			numValidLeases++
		} else {
			numExpiredLeases++
		}
	}
	if numExpiredLeases > 0 {
		return s.writeDynamicLeases()
	}
	if numExpiredLeases > 0 || numValidLeases > 0 {
		s.logger.Printf("read dynamic leases: %d valid and %d expired\n",
			numValidLeases, numExpiredLeases)
	}
	return nil
}

func (s *DhcpServer) removeLease(ipAddr net.IP) {
	if len(ipAddr) < 1 {
		return
	}
	ipStr := ipAddr.String()
	s.mutex.Lock()
	delete(s.staticLeases, s.ipAddrToMacAddr[ipStr])
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
	var subnetToDelete *subnetType
	for _, subnet := range s.subnets {
		if subnet.Id == subnetId {
			subnetToDelete = subnet
		} else {
			subnets = append(subnets, subnet)
		}
	}
	s.subnets = subnets
	if subnetToDelete == nil {
		return
	}
	for name, subnets := range s.interfaceSubnets {
		subnets := make([]*subnetType, 0, len(subnets)-1)
		for _, subnet := range subnets {
			if subnet == subnetToDelete {
				s.logger.Printf("detaching subnet GW: %s from interface: %s\n",
					subnet.IpGateway, name)
			} else {
				subnets = append(subnets, subnet)
			}
		}
		s.interfaceSubnets[name] = subnets
	}
}

func (s *DhcpServer) ServeDHCP(req dhcp.Packet, msgType dhcp.MessageType,
	options dhcp.Options) dhcp.Packet {
	switch msgType {
	case dhcp.Discover:
		macAddr := req.CHAddr().String()
		s.logger.Debugf(1, "Discover from: %s on: %s\n",
			macAddr, s.requestInterface)
		s.mutex.Lock()
		defer s.mutex.Unlock()
		lease, subnet := s.findLease(macAddr, s.requestInterface, nil)
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
			lease.IpAddress, s.computeLeaseTime(lease, true),
			leaseOptions.SelectOrderOrAll(
				options[dhcp.OptionParameterRequestList]))
		packet.SetSIAddr(subnet.myIP)
		return packet
	case dhcp.Request:
		macAddr := req.CHAddr().String()
		reqIP := net.IP(options[dhcp.OptionRequestedIPAddress])
		if reqIP == nil {
			reqIP = net.IP(req.CIAddr())
			s.logger.Debugf(0,
				"Request from: %s on: %s did not request an IP, using: %s\n",
				macAddr, s.requestInterface, reqIP)
		}
		reqIP = util.ShrinkIP(reqIP)
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
		hostname := string(options[dhcp.OptionHostName])
		if hostname != "" {
			s.logger.Debugf(0, "Request for: %s from: %s on: %s HostName=%s\n",
				reqIP, macAddr, s.requestInterface, hostname)
		} else {
			s.logger.Debugf(0, "Request for: %s from: %s on: %s\n",
				reqIP, macAddr, s.requestInterface)
		}
		s.mutex.Lock()
		defer s.mutex.Unlock()
		lease, subnet := s.findLease(macAddr, s.requestInterface, reqIP)
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
			go s.acknowledgeLease(lease.IpAddress)
			lease.clientHostname = hostname
			s.logger.Debugf(0, "ACK for: %s to: %s on: %s, server: %s\n",
				reqIP, macAddr, s.requestInterface, subnet.myIP)
			packet := dhcp.ReplyPacket(req, dhcp.ACK, subnet.myIP, reqIP,
				s.computeLeaseTime(lease, false), leaseOptions.SelectOrderOrAll(
					options[dhcp.OptionParameterRequestList]))
			packet.SetSIAddr(subnet.myIP)
			return packet
		} else {
			s.logger.Debugf(0, "NAK for: %s to: %s on: %s\n",
				reqIP, macAddr, s.requestInterface)
			return dhcp.ReplyPacket(req, dhcp.NAK, subnet.myIP, nil, 0, nil)
		}
	default:
		s.logger.Debugf(0, "Unsupported message type: %s on: %s\n",
			msgType, s.requestInterface)
	}
	return nil
}

// This must be called with the lock held.
func (s *DhcpServer) writeDynamicLeases() error {
	var leases []dynamicLeaseType
	for _, lease := range s.dynamicLeases {
		if time.Until(lease.expires) > 0 {
			leases = append(leases,
				dynamicLeaseType{
					ClientHostName: lease.clientHostname,
					Expires:        lease.expires,
					Address:        lease.Address,
				})
		}
	}
	if len(leases) < 1 {
		os.Remove(s.dynamicLeasesFile)
		return nil
	}
	sort.Slice(leases, func(left, right int) bool {
		return leases[left].Address.IpAddress.String() <
			leases[right].Address.IpAddress.String()
	})
	return json.WriteToFile(s.dynamicLeasesFile, fsutil.PublicFilePerms, "    ",
		leases)
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

// This must be called with the lock held.
func (subnet *subnetType) dynamicOK() bool {
	if !subnet.amGateway {
		return false
	}
	if len(subnet.FirstDynamicIP) < 4 || len(subnet.LastDynamicIP) < 4 {
		return false
	}
	return true
}
