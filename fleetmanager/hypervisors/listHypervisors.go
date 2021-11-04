package hypervisors

import (
	"bufio"
	"fmt"
	"net/http"
	"sort"
	"strconv"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/html"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/url"
	proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
)

const (
	showOK = iota
	showConnected
	showAll
	showOff
)

type hypervisorList []*hypervisorType

func (h *hypervisorType) getHealthStatus() string {
	healthStatus := h.probeStatus.String()
	if h.probeStatus == probeStatusConnected {
		if h.healthStatus != "" {
			healthStatus = h.healthStatus
		}
	}
	return healthStatus
}

func (h *hypervisorType) getNumVMs() uint {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return uint(len(h.vms))
}

func (m *Manager) listHypervisors(topologyDir string, showFilter int,
	subnetId string) (hypervisorList, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	machines, err := m.topology.ListMachines(topologyDir)
	if err != nil {
		return nil, err
	}
	hypervisors := make([]*hypervisorType, 0, len(machines))
	for _, machine := range machines {
		if subnetId != "" {
			hasSubnet, _ := m.topology.CheckIfMachineHasSubnet(
				machine.Hostname, subnetId)
			if !hasSubnet {
				continue
			}
		}
		hypervisor := m.hypervisors[machine.Hostname]
		switch showFilter {
		case showOK:
			if hypervisor.probeStatus == probeStatusConnected &&
				(hypervisor.healthStatus == "" ||
					hypervisor.healthStatus == "healthy") {
				hypervisors = append(hypervisors, hypervisor)
			}
		case showConnected:
			if hypervisor.probeStatus == probeStatusConnected {
				hypervisors = append(hypervisors, hypervisor)
			}
		case showAll:
			hypervisors = append(hypervisors, hypervisor)
		case showOff:
			if hypervisor.probeStatus == probeStatusOff {
				hypervisors = append(hypervisors, hypervisor)
			}
		}
	}
	return hypervisors, nil
}

func (m *Manager) listHypervisorsHandler(w http.ResponseWriter,
	req *http.Request) {
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	_, err := m.getTopology()
	if err != nil {
		fmt.Fprintln(writer, err)
		return
	}
	parsedQuery := url.ParseQuery(req.URL)
	showFilter := showAll
	switch parsedQuery.Table["state"] {
	case "connected":
		showFilter = showConnected
	case "OK":
		showFilter = showOK
	case "off":
		showFilter = showOff
	}
	hypervisors, err := m.listHypervisors("", showFilter, "")
	if err != nil {
		fmt.Fprintln(writer, err)
		return
	}
	sort.Sort(hypervisors)
	if parsedQuery.OutputType() == url.OutputTypeText {
		for _, hypervisor := range hypervisors {
			fmt.Fprintln(writer, hypervisor.machine.Hostname)
		}
		return
	}
	if parsedQuery.OutputType() == url.OutputTypeJson {
		json.WriteWithIndent(writer, "    ", hypervisors)
		return
	}
	fmt.Fprintf(writer, "<title>List of hypervisors</title>\n")
	writer.WriteString(commonStyleSheet)
	fmt.Fprintln(writer, "<body>")
	fmt.Fprintln(writer, `<table border="1" style="width:100%">`)
	tw, _ := html.NewTableWriter(writer, true,
		"Name", "Status", "IP Addr", "Serial Number", "Location", "Type",
		"CPUs", "RAM", "NumVMs")
	for _, hypervisor := range hypervisors {
		machine := hypervisor.machine
		machineType := machine.Tags["Type"]
		if machineTypeURL := machine.Tags["TypeURL"]; machineTypeURL != "" {
			machineType = `<a href="` + machineTypeURL + `">` + machineType +
				`</a>`
		}
		tw.WriteRow("", "",
			fmt.Sprintf("<a href=\"showHypervisor?%s\">%s</a>",
				machine.Hostname, machine.Hostname),
			fmt.Sprintf("<a href=\"http://%s:%d/\">%s</a>",
				machine.Hostname, constants.HypervisorPortNumber,
				hypervisor.getHealthStatus()),
			machine.HostIpAddress.String(),
			hypervisor.serialNumber,
			hypervisor.location,
			machineType,
			strconv.FormatUint(uint64(hypervisor.numCPUs), 10),
			format.FormatBytes(hypervisor.memoryInMiB<<20),
			fmt.Sprintf("<a href=\"http://%s:%d/listVMs\">%d</a>",
				machine.Hostname, constants.HypervisorPortNumber,
				hypervisor.getNumVMs()),
		)
	}
	fmt.Fprintln(writer, "</table>")
	fmt.Fprintln(writer, "</body>")
}

func (m *Manager) listHypervisorsInLocation(
	request proto.ListHypervisorsInLocationRequest) ([]string, error) {
	showFilter := showOK
	if request.IncludeUnhealthy {
		showFilter = showConnected
	}
	hypervisors, err := m.listHypervisors(request.Location, showFilter,
		request.SubnetId)
	if err != nil {
		return nil, err
	}
	addresses := make([]string, 0, len(hypervisors))
	for _, hypervisor := range hypervisors {
		addresses = append(addresses,
			fmt.Sprintf("%s:%d",
				hypervisor.machine.Hostname, constants.HypervisorPortNumber))
	}
	return addresses, nil
}

func (list hypervisorList) Len() int {
	return len(list)
}

func (list hypervisorList) Less(i, j int) bool {
	if list[i].location < list[j].location {
		return true
	} else if list[i].location > list[j].location {
		return false
	} else {
		return list[i].machine.Hostname < list[j].machine.Hostname
	}
}

func (list hypervisorList) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}
