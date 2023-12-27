package hypervisors

import (
	"bufio"
	"fmt"
	"net/http"
	"sort"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/html"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/tags"
	"github.com/Cloud-Foundations/Dominator/lib/tags/tagmatcher"
	"github.com/Cloud-Foundations/Dominator/lib/url"
	proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
)

const (
	showOK = iota
	showConnected
	showDisabled
	showAll
	showOff
)

type hypervisorList []*hypervisorType

func writeHypervisorTotalsStats(hypervisors []*hypervisorType, numVMs uint,
	tw *html.TableWriter) {
	var memoryInMiBAllocated, memoryInMiBTotal uint64
	var milliCPUsAllocated uint64
	var cpusTotal uint
	var volumeBytesAllocated, volumeBytesTotal uint64
	for _, h := range hypervisors {
		memoryInMiBAllocated += h.allocatedMemory
		memoryInMiBTotal += h.memoryInMiB
		milliCPUsAllocated += h.allocatedMilliCPUs
		cpusTotal += h.numCPUs
		volumeBytesAllocated += h.allocatedVolumeBytes
		volumeBytesTotal += h.totalVolumeBytes
	}
	memoryShift, memoryMultiplier := format.GetMiltiplier(
		memoryInMiBAllocated << 20)
	volumeShift, volumeMultiplier := format.GetMiltiplier(
		volumeBytesAllocated)
	tw.WriteRow("", "",
		"<b>TOTAL</b>",
		"",
		"",
		"",
		"",
		"",
		fmt.Sprintf("%.3f/%d", float64(milliCPUsAllocated)/1000, cpusTotal),
		fmt.Sprintf("%d/%d %sB",
			memoryInMiBAllocated<<20>>memoryShift,
			memoryInMiBTotal<<20>>memoryShift,
			memoryMultiplier),
		fmt.Sprintf("%d/%d %sB",
			volumeBytesAllocated>>volumeShift,
			volumeBytesTotal>>volumeShift,
			volumeMultiplier),
		fmt.Sprintf("%d", numVMs))
}

func (h *hypervisorType) getHealthStatus() string {
	healthStatus := h.probeStatus.String()
	if h.probeStatus == probeStatusConnected {
		if h.healthStatus != "" {
			healthStatus = h.healthStatus
		} else if h.disabled {
			healthStatus = "disabled"
		}
	}
	return healthStatus
}

func (h *hypervisorType) getNumVMs() uint {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return uint(len(h.vms))
}

func (h *hypervisorType) writeStats(tw *html.TableWriter) uint {
	machine := h.machine
	machineType := machine.Tags["Type"]
	if machineTypeURL := machine.Tags["TypeURL"]; machineTypeURL != "" {
		machineType = `<a href="` + machineTypeURL + `">` + machineType +
			`</a>`
	}
	numShift := 0
	memoryInMiB := h.memoryInMiB
	for ; memoryInMiB >= 16; numShift++ {
		memoryInMiB >>= 1
	}
	if memoryInMiB == 15 {
		memoryInMiB++
		memoryInMiB <<= numShift
	} else {
		memoryInMiB = h.memoryInMiB
	}
	memoryShift, memoryMultiplier := format.GetMiltiplier(memoryInMiB << 20)
	volumeShift, volumeMultiplier := format.GetMiltiplier(
		h.totalVolumeBytes)
	numVMs := h.getNumVMs()
	tw.WriteRow("", "",
		fmt.Sprintf("<a href=\"showHypervisor?%s\">%s</a>",
			machine.Hostname, machine.Hostname),
		fmt.Sprintf("<a href=\"http://%s:%d/\">%s</a>",
			machine.Hostname, constants.HypervisorPortNumber,
			h.getHealthStatus()),
		machine.HostIpAddress.String(),
		h.serialNumber,
		h.location,
		machineType,
		fmt.Sprintf("%.3f/%d",
			float64(h.allocatedMilliCPUs)/1000,
			h.numCPUs),
		fmt.Sprintf("%d/%d %sB",
			h.allocatedMemory<<20>>memoryShift,
			memoryInMiB<<20>>memoryShift,
			memoryMultiplier),
		fmt.Sprintf("%d/%d %sB",
			h.allocatedVolumeBytes>>volumeShift,
			h.totalVolumeBytes>>volumeShift,
			volumeMultiplier),
		fmt.Sprintf("<a href=\"http://%s:%d/listVMs\">%d</a>",
			machine.Hostname, constants.HypervisorPortNumber,
			numVMs),
	)
	return numVMs
}

func (m *Manager) listHypervisors(topologyDir string, showFilter int,
	subnetId string,
	tagsToMatch *tagmatcher.TagMatcher) (hypervisorList, error) {
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
		if tagsToMatch != nil {
			if !tagsToMatch.MatchEach(machine.Tags) &&
				!tagsToMatch.MatchEach(hypervisor.localTags) {
				continue
			}
		}
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
		case showDisabled:
			if hypervisor.probeStatus == probeStatusConnected &&
				hypervisor.disabled {
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
	case "disabled":
		showFilter = showDisabled
	case "OK":
		showFilter = showOK
	case "off":
		showFilter = showOff
	}
	hypervisors, err := m.listHypervisors("", showFilter, "", nil)
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
		"CPUs", "RAM", "Storage", "NumVMs")
	var numVMs uint
	for _, hypervisor := range hypervisors {
		numVMs += hypervisor.writeStats(tw)
	}
	writeHypervisorTotalsStats(hypervisors, numVMs, tw)
	tw.Close()
	fmt.Fprintln(writer, "</body>")
}

func (m *Manager) listHypervisorsInLocation(
	request proto.ListHypervisorsInLocationRequest) (
	proto.ListHypervisorsInLocationResponse, error) {
	showFilter := showOK
	if request.IncludeUnhealthy {
		showFilter = showConnected
	}
	hypervisors, err := m.listHypervisors(request.Location, showFilter,
		request.SubnetId, tagmatcher.New(request.HypervisorTagsToMatch, false))
	if err != nil {
		return proto.ListHypervisorsInLocationResponse{}, err
	}
	addresses := make([]string, 0, len(hypervisors))
	var tagsForHypervisors []tags.Tags
	for _, hypervisor := range hypervisors {
		addresses = append(addresses,
			fmt.Sprintf("%s:%d",
				hypervisor.machine.Hostname, constants.HypervisorPortNumber))
		if len(request.TagsToInclude) > 0 {
			hypervisorTags := make(tags.Tags)
			for _, key := range request.TagsToInclude {
				if value, ok := hypervisor.machine.Tags[key]; ok {
					hypervisorTags[key] = value
				}
				if value, ok := hypervisor.localTags[key]; ok {
					hypervisorTags[key] = value
				}
			}
			tagsForHypervisors = append(tagsForHypervisors, hypervisorTags)
		}
	}
	return proto.ListHypervisorsInLocationResponse{
		HypervisorAddresses: addresses,
		TagsForHypervisors:  tagsForHypervisors,
	}, nil
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
