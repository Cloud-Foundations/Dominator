//go:build linux
// +build linux

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/prefixlogger"
	libnet "github.com/Cloud-Foundations/Dominator/lib/net"
	"github.com/Cloud-Foundations/Dominator/lib/net/configurator"
	fm_proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
	"github.com/d2g/dhcp4"
	"github.com/d2g/dhcp4client"
	"github.com/pin/tftp"
)

type dhcpResponse struct {
	error  error
	name   string
	packet dhcp4.Packet
}

var (
	tftpFiles = map[string]bool{ // If true, file is required.
		"config.json":         true,
		"imagename":           true,
		"imageserver":         true,
		"storage-layout.json": true,
		"tools-imagename":     false,
	}
	zeroIP = net.IP(make([]byte, 4))
)

func configureLocalNetwork(logger log.DebugLogger) (
	*fm_proto.GetMachineInfoResponse, map[string]net.Interface, string, error) {
	if err := run("ifconfig", "", logger, "lo", "up"); err != nil {
		return nil, nil, "", err
	}
	_, interfaces, err := libnet.ListBroadcastInterfaces(
		libnet.InterfaceTypeEtherNet, logger)
	if err != nil {
		return nil, nil, "", err
	}
	// Raise interfaces so that by the time the OS is installed link status
	// should be stable. This is how we discover connected interfaces.
	if err := raiseInterfaces(interfaces, logger); err != nil {
		return nil, nil, "", err
	}
	machineInfo, activeInterface, err := getConfiguration(interfaces, logger)
	if err != nil {
		return nil, nil, "", err
	}
	return machineInfo, interfaces, activeInterface, nil
}

func dhcpRequest(interfaces map[string]net.Interface,
	logger log.DebugLogger) (string, dhcp4.Packet, error) {
	responseChannel := make(chan dhcpResponse, len(interfaces))
	logger.Println("waiting for carrier and DHCP response for each interface")
	cancelChannel := make(chan struct{})
	for _, iface := range interfaces {
		go dhcpRequestOnInterface(iface, cancelChannel, responseChannel,
			prefixlogger.New(iface.Name+": ", logger))
	}
	timer := time.NewTimer(time.Minute * 5)
	for range interfaces {
		select {
		case response := <-responseChannel:
			if response.error != nil {
				logger.Println(response.error)
				continue
			}
			close(cancelChannel)
			timer.Stop()
			return response.name, response.packet, nil
		case <-timer.C:
			return "", nil, errors.New("timed out waiting for DHCP")
		}
	}
	return "", nil, errors.New("unable to issue DHCP request on any interface")
}

func dhcpRequestOnInterface(iface net.Interface, cancelChannel <-chan struct{},
	responseChannel chan<- dhcpResponse, logger log.DebugLogger) {
	packetSocket, err := dhcp4client.NewPacketSock(iface.Index)
	if err != nil {
		responseChannel <- dhcpResponse{
			error: fmt.Errorf("%s: failed to create DHCP socket: %s",
				iface.Name, err)}
		return
	}
	defer packetSocket.Close()
	client, err := dhcp4client.New(
		dhcp4client.HardwareAddr(iface.HardwareAddr),
		dhcp4client.Connection(packetSocket),
		dhcp4client.Timeout(time.Second*5))
	if err != nil {
		responseChannel <- dhcpResponse{
			error: fmt.Errorf("%s: failed to create DHCP client: %s",
				iface.Name, err)}
		return
	}
	defer client.Close()
	for ; ; time.Sleep(100 * time.Millisecond) {
		select {
		case <-cancelChannel:
			logger.Debugln(1, "cancelling carrier tests")
			return
		default:
		}
		if libnet.TestCarrier(iface.Name) {
			break
		}
	}
	logger.Debugln(1, "carrier detected")
	for ; ; time.Sleep(100 * time.Millisecond) {
		select {
		case <-cancelChannel:
			logger.Debugln(1, "cancelling DHCP attempts")
			return
		default:
		}
		logger.Debugln(1, "DHCP attempt")
		ok, packet, err := client.Request()
		if err != nil {
			logger.Debugf(1, "DHCP failed: %s\n", err)
			continue
		}
		if !ok {
			continue
		}
		if err := processDhcpPacket(packet); err != nil {
			responseChannel <- dhcpResponse{
				error: fmt.Errorf("%s: %s", iface.Name, err)}
			return
		}
		responseChannel <- dhcpResponse{name: iface.Name, packet: packet}
		return
	}
}

func findInterfaceToConfigure(interfaces map[string]net.Interface,
	machineInfo fm_proto.GetMachineInfoResponse, logger log.DebugLogger) (
	net.Interface, net.IP, *hyper_proto.Subnet, error) {
	networkEntries := configurator.GetNetworkEntries(machineInfo)
	hwAddrToInterface := make(map[string]net.Interface, len(interfaces))
	for _, iface := range interfaces {
		hwAddrToInterface[iface.HardwareAddr.String()] = iface
	}
	for _, networkEntry := range networkEntries {
		if len(networkEntry.HostIpAddress) < 1 {
			continue
		}
		iface, ok := hwAddrToInterface[networkEntry.HostMacAddress.String()]
		if !ok {
			continue
		}
		subnet := configurator.FindMatchingSubnet(machineInfo.Subnets,
			networkEntry.HostIpAddress)
		if subnet == nil {
			logger.Printf("no matching subnet for ip=%s\n",
				networkEntry.HostIpAddress)
			continue
		}
		return iface, networkEntry.HostIpAddress, subnet, nil
	}
	return net.Interface{}, nil, nil,
		errors.New("no network interfaces match injected configuration")
}

func getConfiguration(interfaces map[string]net.Interface,
	logger log.DebugLogger) (*fm_proto.GetMachineInfoResponse, string, error) {
	var machineInfo fm_proto.GetMachineInfoResponse
	err := json.ReadFromFile(filepath.Join(*tftpDirectory, "config.json"),
		&machineInfo)
	if err == nil { // Configuration was injected.
		activeInterface, err := setupNetworkFromConfig(interfaces, machineInfo,
			logger)
		if err != nil {
			return nil, "", err
		}
		return &machineInfo, activeInterface, nil
	}
	if !os.IsNotExist(err) {
		return nil, "", err
	}
	tftpServer, activeInterface, err := setupNetworkFromDhcp(interfaces, logger)
	if err != nil {
		return nil, "", err
	}
	if err := os.MkdirAll(*tftpDirectory, fsutil.DirPerms); err != nil {
		return nil, "", err
	}
	if *configurationLoader != "" {
		err := run(*configurationLoader, "", logger, *tftpDirectory,
			activeInterface)
		if err != nil {
			return nil, "", err
		}
		err = json.ReadFromFile(filepath.Join(*tftpDirectory, "config.json"),
			&machineInfo)
		if err != nil {
			return nil, "", err
		}
		logger.Printf("loaded configuration using: %s\n", *configurationLoader)
		return &machineInfo, activeInterface, nil
	}
	if tftpServer == "" {
		return nil, "", errors.New("no TFTP server given")
	}
	if err := loadTftpFiles(tftpServer, logger); err != nil {
		return nil, "", err
	}
	err = json.ReadFromFile(filepath.Join(*tftpDirectory, "config.json"),
		&machineInfo)
	if err != nil {
		return nil, "", err
	}
	return &machineInfo, activeInterface, nil
}

func getTftpServer(packet dhcp4.Packet, options dhcp4.Options,
	logger log.DebugLogger) string {
	if *tftpServerHostname != "" {
		logger.Printf("tftpServer from command-line: %s\n",
			*tftpServerHostname)
		return *tftpServerHostname
	}
	tftpServer := packet.SIAddr()
	if tftpServer.Equal(zeroIP) {
		tftpServer = net.IP(options[dhcp4.OptionTFTPServerName])
		if tftpServer.Equal(zeroIP) {
			return ""
		}
		logger.Printf("tftpServer from OptionTFTPServerName: %s\n", tftpServer)
	} else {
		logger.Printf("tftpServer from SIAddr: %s\n", tftpServer)
	}
	return tftpServer.String()
}

func injectRandomSeed(client *tftp.Client, logger log.DebugLogger) error {
	randomSeed := &bytes.Buffer{}
	if wt, err := client.Receive("random-seed", "octet"); err != nil {
		if strings.Contains(err.Error(), os.ErrNotExist.Error()) {
			return nil
		}
		return err
	} else if _, err := wt.WriteTo(randomSeed); err != nil {
		return err
	}
	if file, err := os.OpenFile("/dev/urandom", os.O_WRONLY, 0); err != nil {
		return err
	} else {
		defer file.Close()
		if nCopied, err := io.Copy(file, randomSeed); err != nil {
			return err
		} else {
			logger.Printf("copied %d bytes of random data\n", nCopied)
		}
	}
	return nil
}

func loadTftpFiles(tftpServer string, logger log.DebugLogger) error {
	client, err := tftp.NewClient(tftpServer + ":69")
	if err != nil {
		return err
	}
	for name, required := range tftpFiles {
		logger.Debugf(1, "downloading: %s\n", name)
		if wt, err := client.Receive(name, "octet"); err != nil {
			if strings.Contains(err.Error(), "does not exist") && !required {
				logger.Debugf(2, "error receiving: %s: %s\n", name, err)
				continue
			}
			return fmt.Errorf("error receiving: %s: %s", name, err)
		} else {
			filename := filepath.Join(*tftpDirectory, name)
			if file, err := create(filename); err != nil {
				return err
			} else {
				defer file.Close()
				if _, err := wt.WriteTo(file); err != nil {
					return fmt.Errorf("error downloading: %s: %s", name, err)
				}
				logger.Debugf(2, "downloaded: %s\n", name)
			}
		}
	}
	return injectRandomSeed(client, logger)
}

func processDhcpPacket(packet dhcp4.Packet) error {
	options := packet.ParseOptions()
	if len(options[dhcp4.OptionRouter]) < 4 {
		return errors.New("ignoring response with no valid router address")
	}
	return nil
}

func raiseInterfaces(interfaces map[string]net.Interface,
	logger log.DebugLogger) error {
	for name := range interfaces {
		if err := run("ifconfig", "", logger, name, "up"); err != nil {
			return err
		}
	}
	return nil
}

func setHostname(optionHostName []byte, logger log.DebugLogger) error {
	if hostname := optionHostName; len(hostname) > 0 {
		hostname = bytes.ToLower(hostname)
		if isValidHostname(hostname) {
			if err := syscall.Sethostname(hostname); err != nil {
				return err
			}
			logger.Printf("set hostname=\"%s\" from DHCP HostName option",
				string(hostname))
			return nil
		}
		logger.Printf("ignoring invalid DHCP HostName option: %s\n",
			string(hostname))
	}
	if hostname := readHostnameFromKernelCmdline(); len(hostname) > 0 {
		hostname = bytes.ToLower(hostname)
		if isValidHostname(hostname) {
			if err := syscall.Sethostname(hostname); err != nil {
				return err
			}
			logger.Printf("set hostname=\"%s\" from hostname= kernel cmdline",
				string(hostname))
			return nil
		}
		logger.Printf("ignoring invalid hostname= from kernel cmdline: %s\n",
			string(hostname))
	}
	return nil
}

func setupNetwork(ifName string, ipAddr net.IP, subnet *hyper_proto.Subnet,
	logger log.DebugLogger) error {
	err := run("ifconfig", "", logger, ifName, ipAddr.String(), "netmask",
		subnet.IpMask.String(), "up")
	if err != nil {
		return err
	}
	err = run("route", "", logger, "add", "default", "gw",
		subnet.IpGateway.String())
	if err != nil {
		e := run("route", "", logger, "del", "default", "gw",
			subnet.IpGateway.String())
		if e != nil {
			return err
		}
		err = run("route", "", logger, "add", "default", "gw",
			subnet.IpGateway.String())
		if err != nil {
			return err
		}
	}
	if !*dryRun {
		if err := configurator.WriteResolvConf("", subnet); err != nil {
			return err
		}
	}
	return nil
}

func setupNetworkFromConfig(interfaces map[string]net.Interface,
	machineInfo fm_proto.GetMachineInfoResponse, logger log.DebugLogger) (
	string, error) {
	iface, ipAddr, subnet, err := findInterfaceToConfigure(interfaces,
		machineInfo, logger)
	if err != nil {
		return "", err
	}
	if err := setupNetwork(iface.Name, ipAddr, subnet, logger); err != nil {
		return "", err
	}
	return iface.Name, nil
}

func setupNetworkFromDhcp(interfaces map[string]net.Interface,
	logger log.DebugLogger) (string, string, error) {
	ifName, packet, err := dhcpRequest(interfaces, logger)
	if err != nil {
		return "", "", err
	}
	ipAddr := packet.YIAddr()
	options := packet.ParseOptions()
	logger.Printf("%s: using DHCP response with address: %s\n", ifName, ipAddr)
	if logdir, err := logDhcpPacket(ifName, packet, options); err != nil {
		logger.Printf("error logging DHCP packet: %w", err)
	} else {
		logger.Printf("logged DHCP response in: %s\n", logdir)
	}
	if err := setHostname(options[dhcp4.OptionHostName], logger); err != nil {
		return "", "", err
	}
	subnet := hyper_proto.Subnet{
		IpGateway: net.IP(options[dhcp4.OptionRouter]),
		IpMask:    net.IP(options[dhcp4.OptionSubnetMask]),
	}
	dnsServersBuffer := options[dhcp4.OptionDomainNameServer]
	for len(dnsServersBuffer) > 0 {
		if len(dnsServersBuffer) >= 4 {
			subnet.DomainNameServers = append(subnet.DomainNameServers,
				net.IP(dnsServersBuffer[:4]))
			dnsServersBuffer = dnsServersBuffer[4:]
		} else {
			return "", "", errors.New("truncated DNS server address")
		}
	}
	if domainName := options[dhcp4.OptionDomainName]; len(domainName) > 0 {
		subnet.DomainName = string(domainName)
	}
	if err := setupNetwork(ifName, ipAddr, &subnet, logger); err != nil {
		return "", "", err
	}
	tftpServer := getTftpServer(packet, options, logger)
	return tftpServer, ifName, nil
}
