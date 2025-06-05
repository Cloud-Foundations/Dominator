package main

import (
	"github.com/Cloud-Foundations/Dominator/lib/log"
	libnet "github.com/Cloud-Foundations/Dominator/lib/net"
	"github.com/d2g/dhcp4"
)

func dhcpRequestSubcommand(args []string, logger log.DebugLogger) error {
	_, interfaces, err := libnet.ListBroadcastInterfaces(
		libnet.InterfaceTypeEtherNet, logger)
	if err != nil {
		return err
	}
	ifName, packet, err := dhcpRequest(interfaces, false, logger)
	if err != nil {
		return err
	}
	options := packet.ParseOptions()
	if logdir, err := logDhcpPacket(ifName, packet, options); err != nil {
		logger.Printf("error logging DHCP packet: %w", err)
	} else {
		logger.Printf("logged DHCP response in: %s\n", logdir)
	}
	if hostname := options[dhcp4.OptionHostName]; len(hostname) > 0 {
		logger.Printf("DHCP HostName option found, value=%s", string(hostname))
	}
	return nil
}
