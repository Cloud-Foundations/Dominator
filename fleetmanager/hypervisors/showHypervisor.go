package hypervisors

import (
	"bufio"
	"fmt"
	"time"

	"io"
	"net"
	"net/http"
	"sort"

	"github.com/Cloud-Foundations/Dominator/fleetmanager/topology"
	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/html"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/url"
	"github.com/Cloud-Foundations/Dominator/lib/verstr"
	fm_proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

type hypervisorSubnetsType struct {
	Hypervisor *fm_proto.Hypervisor
	Subnets    []*topology.Subnet
}

func (m *Manager) showHypervisorHandler(w http.ResponseWriter,
	req *http.Request) {
	parsedQuery := url.ParseQuery(req.URL)
	if len(parsedQuery.Flags) != 1 {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var hostname string
	for name := range parsedQuery.Flags {
		hostname = name
	}
	h, err := m.getLockedHypervisor(hostname, false)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer h.mutex.RUnlock()
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	topology, err := m.getTopology()
	if err != nil {
		fmt.Fprintln(writer, err)
		return
	}
	if parsedQuery.OutputType() == url.OutputTypeJson {
		subnets, _ := topology.GetSubnetsForMachine(hostname)
		json.WriteWithIndent(writer, "    ", hypervisorSubnetsType{
			Hypervisor: &h.Hypervisor,
			Subnets:    subnets,
		})
		return
	}
	fmt.Fprintf(writer, "<title>Information for Hypervisor %s</title>\n",
		hostname)
	writer.WriteString(commonStyleSheet)
	fmt.Fprintln(writer, "<body>")
	fmt.Fprintln(writer, "Machine info:<br>")
	fmt.Fprintln(writer, `<pre style="background-color: #eee; border: 1px solid #999; display: block; float: left;">`)
	json.WriteWithIndent(writer, "    ", h.Machine)
	fmt.Fprintln(writer, `</pre><p style="clear: both;">`)
	subnets, err := topology.GetSubnetsForMachine(hostname)
	if err != nil {
		fmt.Fprintf(writer, "%s<br>\n", err)
	} else {
		fmt.Fprintln(writer, "Subnets:<br>")
		fmt.Fprintln(writer, `<pre style="background-color: #eee; border: 1px solid #999; display: block; float: left;">`)
		json.WriteWithIndent(writer, "    ", subnets)
		fmt.Fprintln(writer, `</pre><p style="clear: both;">`)
	}
	if !*manageHypervisors {
		fmt.Fprintln(writer, "No visibility into local tags<br>")
	} else if len(h.localTags) > 0 {
		keys := make([]string, 0, len(h.localTags))
		for key := range h.localTags {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		fmt.Fprintln(writer, "Local tags:<br>")
		fmt.Fprintln(writer, `<table border="1">`)
		tw, _ := html.NewTableWriter(writer, true, "Name", "Value")
		for _, key := range keys {
			tw.WriteRow("", "", key, h.localTags[key])
		}
		tw.Close()
	}
	fmt.Fprintf(writer, "Status: %s", h.getHealthStatus(true))
	h.mutex.RLock()
	lastConnectedTime := h.lastConnectedTime
	numVMs := len(h.vms)
	h.mutex.RUnlock()
	if lastConnectedTime.IsZero() {
		fmt.Fprintln(writer, "<br>")
	} else {
		fmt.Fprintf(writer, ", last received: %s (%s ago)<br>\n",
			lastConnectedTime.Format(format.TimeFormatSeconds),
			format.Duration(time.Since(lastConnectedTime)))
	}
	if h.IPMI.Hostname != "" {
		fmt.Fprintf(writer, "<a href=\"https://%s/\">IPMI</a><br>\n",
			h.IPMI.Hostname)
	}
	if h.serialNumber != "" {
		fmt.Fprintf(writer, "Serial Number: %s<br>\n", h.serialNumber)
	}
	fmt.Fprintf(writer,
		"Number of VMs known: %d (<a href=\"http://%s:%d/listVMs\">live view</a>)<br>\n",
		numVMs, hostname, constants.HypervisorPortNumber)
	fmt.Fprintln(writer, "<br>")
	m.showVMsForHypervisor(writer, h)
	fmt.Fprintln(writer, "<br>")
	m.showIPsForHypervisor(writer, h.Machine.HostIpAddress)
	fmt.Fprintln(writer, "</body>")
}

func (m *Manager) showIPsForHypervisor(writer io.Writer, hIP net.IP) {
	if !*manageHypervisors {
		fmt.Fprintln(writer, "No visibility into registered addresses<br>")
	} else if ips, err := m.storer.GetIPsForHypervisor(hIP); err != nil {
		fmt.Fprintf(writer, "Error getting IPs for Hypervisor: %s: %s<br>\n",
			hIP, err)
		return
	} else {
		fmt.Fprintln(writer, "Registered addresses:<br>")
		ipList := make([]string, 0, len(ips))
		for _, ip := range ips {
			ipList = append(ipList, ip.String())
		}
		verstr.Sort(ipList)
		for _, ip := range ipList {
			fmt.Fprintln(writer, ip, "<br>")
		}
	}
}

func (m *Manager) showVMsForHypervisor(writer *bufio.Writer,
	h *hypervisorType) {
	fmt.Fprintln(writer, "VMs as of last update:<br>")
	capacity := hyper_proto.GetCapacityResponse{
		MemoryInMiB:      h.MemoryInMiB,
		NumCPUs:          h.NumCPUs,
		TotalVolumeBytes: h.TotalVolumeBytes,
	}
	h.mutex.RLock()
	vms := getVmListFromMap(h.vms, true)
	h.mutex.RUnlock()
	err := m.listVMs(writer, vms, &capacity, "", "", url.OutputTypeHtml)
	if err != nil {
		fmt.Fprintln(writer, err)
		return
	}
	fmt.Fprintln(writer, "<br>")
	fmt.Fprintln(writer, "VMs by primary owner as of last update:<br>")
	err = m.listVMsByPrimaryOwner(writer, vms, url.OutputTypeHtml)
	if err != nil {
		fmt.Fprintln(writer, err)
		return
	}
}
