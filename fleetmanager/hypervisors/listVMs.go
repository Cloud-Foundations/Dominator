package hypervisors

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/html"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
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

func (m *Manager) listVMs(writer *bufio.Writer, vms []*vmInfoType,
	primaryOwnerFilter string, outputType uint) (map[string]struct{}, error) {
	topology, err := m.getTopology()
	if err != nil {
		return nil, err
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
	primaryOwnersMap := make(map[string]struct{})
	for _, vm := range vms {
		if primaryOwnerFilter != "" {
			if vm.OwnerUsers[0] != primaryOwnerFilter {
				primaryOwnersMap[vm.OwnerUsers[0]] = struct{}{}
				continue
			}
		} else {
			primaryOwnersMap[vm.OwnerUsers[0]] = struct{}{}
		}
		switch outputType {
		case url.OutputTypeText:
			fmt.Fprintln(writer, vm.ipAddr)
		case url.OutputTypeHtml:
			var background, foreground string
			if vm.hypervisor.probeStatus == probeStatusOff {
				foreground = "#ff8080"
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
				fmt.Sprintf("<a href=\"http://%s:%d/showVM?%s\">%s</a>",
					vm.hypervisor.machine.Hostname,
					constants.HypervisorPortNumber, vm.ipAddr, vm.ipAddr),
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
		fmt.Fprintln(writer, "</table>")
	}
	return primaryOwnersMap, nil
}

func (m *Manager) getVMs(doSort bool) []*vmInfoType {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return getVmListFromMap(m.vms, doSort)
}

func (m *Manager) listVMsHandler(w http.ResponseWriter,
	req *http.Request) {
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	parsedQuery := url.ParseQuery(req.URL)
	vms := m.getVMs(true)
	if parsedQuery.OutputType() == url.OutputTypeJson {
		json.WriteWithIndent(writer, "   ", vms)
	}
	primaryOwnerFilter := parsedQuery.Table["primaryOwner"]
	primaryOwnersMap, err := m.listVMs(writer, vms, primaryOwnerFilter,
		parsedQuery.OutputType())
	if err != nil {
		fmt.Fprintln(writer, err)
		return
	}
	switch parsedQuery.OutputType() {
	case url.OutputTypeHtml:
		fmt.Fprintln(writer, "</body>")
		primaryOwners := stringutil.ConvertMapKeysToList(primaryOwnersMap, true)
		fmt.Fprintln(writer, "Filter by primary owner:<br>")
		for _, primaryOwner := range primaryOwners {
			fmt.Fprintf(writer,
				"<a href=\"listVMs?primaryOwner=%s\">%s</a><br>\n",
				primaryOwner, primaryOwner)
		}
	}
}

func (m *Manager) listVMsInLocation(request fm_proto.ListVMsInLocationRequest) (
	[]net.IP, error) {
	hypervisors, err := m.listHypervisors(request.Location, showAll, "")
	if err != nil {
		return nil, err
	}
	ownerGroups := stringutil.ConvertListToMap(request.OwnerGroups, false)
	ownerUsers := stringutil.ConvertListToMap(request.OwnerUsers, false)
	addresses := make([]net.IP, 0)
	for _, hypervisor := range hypervisors {
		hypervisor.mutex.RLock()
		for _, vm := range hypervisor.vms {
			if vm.checkOwnerGroups(ownerGroups) &&
				vm.checkOwnerUsers(ownerUsers) {
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
