package hypervisors

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/html"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
	"github.com/Cloud-Foundations/Dominator/lib/tags/tagmatcher"
	"github.com/Cloud-Foundations/Dominator/lib/url"
	"github.com/Cloud-Foundations/Dominator/lib/verstr"
	fm_proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

const commonStyleSheet string = `<style>
table, th, td {
border-collapse: collapse;
}
</style>
`

type ownerTotalsType struct {
	MemoryInMiB uint64
	MilliCPUs   uint
	NumVMs      uint
	NumVolumes  uint
	VirtualCPUs uint
	VolumeSize  uint64
}

func getTotalsByOwner(vms []*vmInfoType) map[string]*ownerTotalsType {
	totalsByOwner := make(map[string]*ownerTotalsType)
	for _, vm := range vms {
		ownerTotals := totalsByOwner[vm.OwnerUsers[0]]
		if ownerTotals == nil {
			ownerTotals = &ownerTotalsType{}
			totalsByOwner[vm.OwnerUsers[0]] = ownerTotals
		}
		ownerTotals.MemoryInMiB += vm.MemoryInMiB
		ownerTotals.MilliCPUs += vm.MilliCPUs
		ownerTotals.NumVMs++
		ownerTotals.NumVolumes += uint(len(vm.Volumes))
		if vm.VirtualCPUs < 1 {
			ownerTotals.VirtualCPUs++
		} else {
			ownerTotals.VirtualCPUs += vm.VirtualCPUs
		}
		for _, volume := range vm.Volumes {
			ownerTotals.VolumeSize += volume.Size
		}
	}
	return totalsByOwner
}

func getVmListFromMap(vmMap map[string]*vmInfoType, doSort bool) []*vmInfoType {
	vms := make([]*vmInfoType, 0, len(vmMap))
	if doSort {
		ipAddrs := make([]string, 0, len(vmMap))
		for ipAddr := range vmMap {
			ipAddrs = append(ipAddrs, ipAddr)
		}
		verstr.Sort(ipAddrs)
		for _, ipAddr := range ipAddrs {
			vms = append(vms, vmMap[ipAddr])
		}
	} else {
		for _, vm := range vmMap {
			vms = append(vms, vm)
		}
	}
	return vms
}

// numSpecifiedVirtualCPUs calculates the number of virtual CPUs required for
// the specified request. The request must be correct (i.e. sufficient vCPUs).
func numSpecifiedVirtualCPUs(milliCPUs, vCPUs uint) uint {
	nCpus := milliCPUs / 1000
	if nCpus < 1 {
		nCpus = 1
	}
	if nCpus*1000 < milliCPUs {
		nCpus++
	}
	if nCpus < vCPUs {
		nCpus = vCPUs
	}
	return nCpus
}

func sumOwnerTotals(totalsByOwner map[string]*ownerTotalsType) ownerTotalsType {
	var totals ownerTotalsType
	for _, ownerTotals := range totalsByOwner {
		totals.MemoryInMiB += ownerTotals.MemoryInMiB
		totals.MilliCPUs += ownerTotals.MilliCPUs
		totals.NumVMs += ownerTotals.NumVMs
		totals.NumVolumes += ownerTotals.NumVolumes
		totals.VirtualCPUs += ownerTotals.VirtualCPUs
		totals.VolumeSize += ownerTotals.VolumeSize

	}
	return totals
}

func (m *Manager) getVMs(doSort bool) []*vmInfoType {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return getVmListFromMap(m.vms, doSort)
}

func (m *Manager) listVMs(writer *bufio.Writer, vms []*vmInfoType,
	primaryOwnerFilter string, outputType uint) error {
	topology, err := m.getTopology()
	if err != nil {
		return err
	}
	var tw *html.TableWriter
	if outputType == url.OutputTypeHtml {
		fmt.Fprintf(writer, "<title>List of VMs</title>\n")
		writer.WriteString(commonStyleSheet)
		fmt.Fprintln(writer, `<table border="1" style="width:100%">`)
		tw, _ = html.NewTableWriter(writer, true, "IP Addr", "Name(tag)",
			"State", "RAM", "CPU", "vCPU", "Num Volumes", "Storage",
			"Primary Owner", "Hypervisor", "Location")
	}
	var vmsToShow []*vmInfoType
	for _, vm := range vms {
		if primaryOwnerFilter != "" &&
			vm.OwnerUsers[0] != primaryOwnerFilter {
			continue
		}
		vmsToShow = append(vmsToShow, vm)
		switch outputType {
		case url.OutputTypeText:
			fmt.Fprintln(writer, vm.ipAddr)
		case url.OutputTypeHtml:
			var background, foreground string
			if vm.hypervisor.probeStatus == probeStatusOff {
				foreground = "#ff8080"
			} else if vm.hypervisor.probeStatus == probeStatusConnected &&
				vm.hypervisor.disabled {
				foreground = "grey"
			} else if vm.hypervisor.probeStatus != probeStatusConnected {
				foreground = "red"
			} else if vm.hypervisor.healthStatus == "at risk" {
				foreground = "#c00000"
			} else if vm.hypervisor.healthStatus == "marginal" {
				foreground = "#800000"
			}
			if vm.Uncommitted {
				background = "yellow"
			} else if topology.CheckIfIpIsHost(vm.ipAddr) ||
				topology.CheckIfIpIsReserved(vm.ipAddr) {
				background = "orange"
			}
			vCPUs := strconv.Itoa(int(numSpecifiedVirtualCPUs(vm.MilliCPUs,
				vm.VirtualCPUs)))
			if vm.VirtualCPUs < 1 {
				vCPUs = `<font color="grey">` + vCPUs + `</font>`
			}
			tw.WriteRow(foreground, background,
				fmt.Sprintf("<a href=\"showVM?%s\">%s</a>",
					vm.ipAddr, vm.ipAddr),
				vm.Tags["Name"],
				vm.State.String(),
				format.FormatBytes(vm.MemoryInMiB<<20),
				fmt.Sprintf("%g", float64(vm.MilliCPUs)*1e-3),
				vCPUs,
				vm.numVolumesTableEntry(),
				vm.storageTotalTableEntry(),
				vm.OwnerUsers[0],
				fmt.Sprintf("<a href=\"http://%s:%d/\">%s</a>",
					vm.hypervisor.machine.Hostname,
					constants.HypervisorPortNumber,
					vm.hypervisor.machine.Hostname),
				vm.hypervisor.location,
			)
		}
	}
	switch outputType {
	case url.OutputTypeHtml:
		totalsByOwner := getTotalsByOwner(vmsToShow)
		totals := sumOwnerTotals(totalsByOwner)
		tw.WriteRow("", "",
			"<b>TOTAL</b>",
			"",
			"",
			format.FormatBytes(totals.MemoryInMiB<<20),
			fmt.Sprintf("%g", float64(totals.MilliCPUs)*1e-3),
			strconv.FormatUint(uint64(totals.VirtualCPUs), 10),
			strconv.FormatUint(uint64(totals.NumVolumes), 10),
			format.FormatBytes(totals.VolumeSize),
			primaryOwnerFilter,
			"",
			"")
		tw.Close()
	case url.OutputTypeJson:
		json.WriteWithIndent(writer, "   ", vmsToShow)
	}
	return nil
}

func (m *Manager) listVMsByPrimaryOwnerHandler(w http.ResponseWriter,
	req *http.Request) {
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	parsedQuery := url.ParseQuery(req.URL)
	vms := m.getVMs(true)
	totalsByOwner := getTotalsByOwner(vms)
	ownersList := make([]string, 0, len(totalsByOwner))
	for owner := range totalsByOwner {
		ownersList = append(ownersList, owner)
	}
	sort.Strings(ownersList)
	switch parsedQuery.OutputType() {
	case url.OutputTypeHtml:
		fmt.Fprintf(writer, "<title>List of VMs By Primary Owner</title>\n")
		writer.WriteString(commonStyleSheet)
		fmt.Fprintln(writer, `<table border="1" style="width:100%">`)
		tw, _ := html.NewTableWriter(writer, true, "Owner", "Num VMs", "RAM",
			"CPU", "vCPU", "Num Volumes", "Storage")
		for _, owner := range ownersList {
			ownerTotals := totalsByOwner[owner]
			tw.WriteRow("", "",
				fmt.Sprintf("<a href=\"listVMs?primaryOwner=%s\">%s</a>",
					owner, owner),
				strconv.FormatUint(uint64(ownerTotals.NumVMs), 10),
				format.FormatBytes(ownerTotals.MemoryInMiB<<20),
				fmt.Sprintf("%g", float64(ownerTotals.MilliCPUs)*1e-3),
				strconv.FormatUint(uint64(ownerTotals.VirtualCPUs), 10),
				strconv.FormatUint(uint64(ownerTotals.NumVolumes), 10),
				format.FormatBytes(ownerTotals.VolumeSize))
		}
		totals := sumOwnerTotals(totalsByOwner)
		tw.WriteRow("", "",
			"<b>TOTAL</b>",
			strconv.FormatUint(uint64(totals.NumVMs), 10),
			format.FormatBytes(totals.MemoryInMiB<<20),
			fmt.Sprintf("%g", float64(totals.MilliCPUs)*1e-3),
			strconv.FormatUint(uint64(totals.VirtualCPUs), 10),
			strconv.FormatUint(uint64(totals.NumVolumes), 10),
			format.FormatBytes(totals.VolumeSize))
		tw.Close()
		fmt.Fprintln(writer, "</body>")
	case url.OutputTypeJson:
		json.WriteWithIndent(writer, "   ", totalsByOwner)
	case url.OutputTypeText:
		for _, owner := range ownersList {
			ownerTotals := totalsByOwner[owner]
			fmt.Fprintf(writer, "%s %d\n", owner, ownerTotals.NumVMs)
		}
	}
}

func (m *Manager) listVMsHandler(w http.ResponseWriter,
	req *http.Request) {
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	parsedQuery := url.ParseQuery(req.URL)
	vms := m.getVMs(true)
	primaryOwnerFilter := parsedQuery.Table["primaryOwner"]
	err := m.listVMs(writer, vms, primaryOwnerFilter, parsedQuery.OutputType())
	if err != nil {
		fmt.Fprintln(writer, err)
		return
	}
	switch parsedQuery.OutputType() {
	case url.OutputTypeHtml:
		fmt.Fprintln(writer, "</body>")
	}
}

func (m *Manager) listVMsInLocation(request fm_proto.ListVMsInLocationRequest) (
	[]net.IP, error) {
	hypervisors, err := m.listHypervisors(request.Location, showAll, "",
		tagmatcher.New(request.HypervisorTagsToMatch, false))
	if err != nil {
		return nil, err
	}
	ownerGroups := stringutil.ConvertListToMap(request.OwnerGroups, false)
	ownerUsers := stringutil.ConvertListToMap(request.OwnerUsers, false)
	addresses := make([]net.IP, 0)
	vmTagMatcher := tagmatcher.New(request.VmTagsToMatch, false)
	for _, hypervisor := range hypervisors {
		hypervisor.mutex.RLock()
		for _, vm := range hypervisor.vms {
			if vm.checkOwnerGroups(ownerGroups) &&
				vm.checkOwnerUsers(ownerUsers) &&
				vmTagMatcher.MatchEach(vm.Tags) {
				addresses = append(addresses, vm.Address.IpAddress)
			}
		}
		hypervisor.mutex.RUnlock()
	}
	return addresses, nil
}

// checkOwnerGroups returns true if one of the specified ownerGroups owns the
// VM. If ownerGroups is nil, checkOwnerGroups returns true.
func (vm *vmInfoType) checkOwnerGroups(ownerGroups map[string]struct{}) bool {
	if ownerGroups == nil {
		return true
	}
	for _, ownerGroup := range vm.OwnerGroups {
		if _, ok := ownerGroups[ownerGroup]; ok {
			return true
		}
	}
	return false
}

// checkOwnerUsers returns true if one of the specified ownerUsers owns the VM.
// If ownerUsers is nil, checkOwnerUsers returns true.
func (vm *vmInfoType) checkOwnerUsers(ownerUsers map[string]struct{}) bool {
	if ownerUsers == nil {
		return true
	}
	for _, ownerUser := range vm.OwnerUsers {
		if _, ok := ownerUsers[ownerUser]; ok {
			return true
		}
	}
	return false
}

func (vm *vmInfoType) numVolumesTableEntry() string {
	var comment string
	for _, volume := range vm.Volumes {
		if comment == "" && volume.Format != hyper_proto.VolumeFormatRaw {
			comment = `<font style="color:grey;font-size:12px"> (!RAW)</font>`
		}
	}
	return fmt.Sprintf("%d%s", len(vm.Volumes), comment)
}

func (vm *vmInfoType) storageTotalTableEntry() string {
	var storage uint64
	for _, volume := range vm.Volumes {
		storage += volume.Size
	}
	return format.FormatBytes(storage)
}
