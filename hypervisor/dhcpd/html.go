package dhcpd

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/html"
)

func (s *DhcpServer) writeHtml(writer io.Writer) {
	fmt.Fprintln(writer,
		`DHCP server <a href="showDhcpStatus">status</a><br>`)
}

func (s *DhcpServer) showDhcpStatusHandler(w http.ResponseWriter,
	req *http.Request) {
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	fmt.Fprintln(writer, "<title>hypervisor DHCP server status page</title>")
	fmt.Fprintln(writer, `<style>
                          table, th, td {
                          border-collapse: collapse;
                          }
                          </style>`)
	fmt.Fprintln(writer, "<body>")
	fmt.Fprintln(writer, "<center>")
	fmt.Fprintln(writer, "<h1>hypervisor DHCP server status page</h1>")
	fmt.Fprintln(writer, "</center>")
	s.writeDashboard(writer)
	fmt.Fprintln(writer, "<hr>")
	html.WriteFooter(writer)
	fmt.Fprintln(writer, "</body>")
}

func (s *DhcpServer) writeDashboard(writer io.Writer) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	fmt.Fprintln(writer, "<b>Interfaces</b><br>")
	fmt.Fprintln(writer, `<table border="1">`)
	tw, _ := html.NewTableWriter(writer, true, "Interface", "IPs")
	for interfaceName, IPs := range s.interfaceIPs {
		tw.WriteRow("", "", interfaceName, fmt.Sprintf("%v", IPs))
	}
	fmt.Fprintln(writer, "</table><br>")

	fmt.Fprintln(writer, "<b>Route Table</b><br>")
	fmt.Fprintln(writer, `<table border="1">`)
	tw, _ = html.NewTableWriter(writer, true,
		"Interface", "Base", "Broadcast", "Gateway", "Mask")
	for interfaceName, entry := range s.routeTable {
		tw.WriteRow("", "",
			interfaceName, entry.BaseAddr.String(),
			entry.BroadcastAddr.String(),
			entry.GatewayAddr.String(), entry.Mask.String())
	}
	fmt.Fprintln(writer, "</table><br>")

	fmt.Fprintln(writer, "<b>Static leases</b><br>")
	fmt.Fprintln(writer, `<table border="1">`)
	tw, _ = html.NewTableWriter(writer, true,
		"MAC", "IP", "Hostname", "SubnetID")
	for macAddr, lease := range s.staticLeases {
		tw.WriteRow("", "", macAddr, lease.IpAddress.String(), lease.hostname,
			lease.subnet.Id)
	}
	fmt.Fprintln(writer, "</table><br>")

	fmt.Fprintln(writer, "<b>Dynamic leases</b><br>")
	fmt.Fprintln(writer, `<table border="1">`)
	tw, _ = html.NewTableWriter(writer, true,
		"MAC", "IP", "Client Hostname", "SubnetID", "Expires")
	for macAddr, lease := range s.dynamicLeases {
		var subnetId string
		if lease.subnet != nil {
			subnetId = lease.subnet.Id
		}
		tw.WriteRow("", "", macAddr, lease.IpAddress.String(),
			lease.clientHostname, subnetId,
			lease.expires.Round(time.Second).String())
	}
	fmt.Fprintln(writer, "</table><br>")
}
