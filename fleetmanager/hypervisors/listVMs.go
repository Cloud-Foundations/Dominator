package hypervisors

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"sort"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/html"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/url"
	"github.com/Cloud-Foundations/Dominator/lib/verstr"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
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
			"State", "RAM", "CPU", "Num Volumes", "Storage", "Primary Owner",
			"Hypervisor", "Location")
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
			} else if topology.CheckIfIpIsReserved(vm.ipAddr) {
				background = "orange"
			}
			tw.WriteRow(foreground, background,
				fmt.Sprintf("<a href=\"http://%s:%d/showVM?%s\">%s</a>",
					vm.hypervisor.machine.Hostname,
					constants.HypervisorPortNumber, vm.ipAddr, vm.ipAddr),
				vm.Tags["Name"],
				vm.State.String(),
				format.FormatBytes(vm.MemoryInMiB<<20),
				fmt.Sprintf("%g", float64(vm.MilliCPUs)*1e-3),
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
		primaryOwners := make([]string, 0, len(primaryOwnersMap))
		for primaryOwner := range primaryOwnersMap {
			primaryOwners = append(primaryOwners, primaryOwner)
		}
		sort.Strings(primaryOwners)
		fmt.Fprintln(writer, "Filter by primary owner:<br>")
		for _, primaryOwner := range primaryOwners {
			fmt.Fprintf(writer,
				"<a href=\"listVMs?primaryOwner=%s\">%s</a><br>\n",
				primaryOwner, primaryOwner)
		}
	}
}

func (m *Manager) listVMsInLocation(dirname string) ([]net.IP, error) {
	hypervisors, err := m.listHypervisors(dirname, showAll, "")
	if err != nil {
		return nil, err
	}
	addresses := make([]net.IP, 0)
	for _, hypervisor := range hypervisors {
		hypervisor.mutex.RLock()
		for _, vm := range hypervisor.vms {
			addresses = append(addresses, vm.Address.IpAddress)
		}
		hypervisor.mutex.RUnlock()
	}
	return addresses, nil
}

func (vm *vmInfoType) numVolumesTableEntry() string {
	var comment string
	for _, volume := range vm.Volumes {
		if comment == "" && volume.Format != proto.VolumeFormatRaw {
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
