package httpd

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/html"
	"github.com/Cloud-Foundations/Dominator/lib/url"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

type ownerTotalsType struct {
	MemoryInMiB uint64
	MilliCPUs   uint
	NumVMs      uint
	NumVolumes  uint
	VirtualCPUs uint
	VolumeSize  uint64
}

func addVmToOwnersTotals(totalsByOwner map[string]*ownerTotalsType,
	vm *proto.VmInfo) {
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

func listVMsByPrimaryOwner(writer io.Writer,
	totalsByOwner map[string]*ownerTotalsType) error {
	ownersList := make([]string, 0, len(totalsByOwner))
	for owner := range totalsByOwner {
		ownersList = append(ownersList, owner)
	}
	sort.Strings(ownersList)
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
			format.FormatMilli(uint64(ownerTotals.MilliCPUs)),
			strconv.FormatUint(uint64(ownerTotals.VirtualCPUs), 10),
			strconv.FormatUint(uint64(ownerTotals.NumVolumes), 10),
			format.FormatBytes(ownerTotals.VolumeSize))
	}
	totals := sumOwnerTotals(totalsByOwner)
	tw.WriteRow("", "",
		"<b>TOTAL</b>",
		strconv.FormatUint(uint64(totals.NumVMs), 10),
		format.FormatBytes(totals.MemoryInMiB<<20),
		format.FormatMilli(uint64(totals.MilliCPUs)),
		strconv.FormatUint(uint64(totals.VirtualCPUs), 10),
		strconv.FormatUint(uint64(totals.NumVolumes), 10),
		format.FormatBytes(totals.VolumeSize))
	tw.Close()
	return nil
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

func (s state) listVMsHandler(w http.ResponseWriter, req *http.Request) {
	parsedQuery := url.ParseQuery(req.URL)
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	ipAddrs := s.manager.ListVMs(proto.ListVMsRequest{Sort: true})
	matchState := parsedQuery.Table["state"]
	if parsedQuery.OutputType() == url.OutputTypeText && matchState == "" {
		for _, ipAddr := range ipAddrs {
			fmt.Fprintln(writer, ipAddr)
		}
		return
	}
	var tw *html.TableWriter
	if parsedQuery.OutputType() == url.OutputTypeHtml {
		fmt.Fprintf(writer, "<title>List of VMs</title>\n")
		fmt.Fprintln(writer, `<style>
                          table, th, td {
                          border-collapse: collapse;
                          }
                          </style>`)
		fmt.Fprintln(writer, "<body>")
		fmt.Fprintln(writer, `<table border="1" style="width:100%">`)
		tw, _ = html.NewTableWriter(writer, true, "IP Addr", "MAC Addr",
			"Name(tag)", "State", "RAM", "CPU", "vCPU", "Num Volumes",
			"Storage", "Primary Owner")
	}
	var allocatedMemoryInMiB uint64
	var allocatedMilliCPUs, allocatedVirtualCPUs, numVolumes uint
	var allocatedVolumeSize uint64
	totalsByOwner := make(map[string]*ownerTotalsType)
	for _, ipAddr := range ipAddrs {
		vm, err := s.manager.GetVmInfo(net.ParseIP(ipAddr))
		if err != nil {
			continue
		}
		if matchState != "" && matchState != vm.State.String() {
			continue
		}
		switch parsedQuery.OutputType() {
		case url.OutputTypeText:
			fmt.Fprintln(writer, ipAddr)
		case url.OutputTypeHtml:
			allocatedMemoryInMiB += vm.MemoryInMiB
			allocatedMilliCPUs += vm.MilliCPUs
			var background string
			if vm.Uncommitted {
				background = "yellow"
			}
			vCPUs := strconv.Itoa(int(numSpecifiedVirtualCPUs(vm.MilliCPUs,
				vm.VirtualCPUs)))
			if vm.VirtualCPUs < 1 {
				vCPUs = `<font color="grey">` + vCPUs + `</font>`
				allocatedVirtualCPUs++
			} else {
				allocatedVirtualCPUs += vm.VirtualCPUs
			}
			numVolumes += uint(len(vm.Volumes))
			volumeSize, volumeString := storageTotalTableEntry(vm)
			allocatedVolumeSize += volumeSize
			tw.WriteRow("", background,
				fmt.Sprintf("<a href=\"showVM?%s\">%s</a>", ipAddr, ipAddr),
				vm.Address.MacAddress,
				vm.Tags["Name"],
				vm.State.String(),
				format.FormatBytes(vm.MemoryInMiB<<20),
				format.FormatMilli(uint64(vm.MilliCPUs)),
				vCPUs,
				numVolumesTableEntry(vm),
				volumeString,
				vm.OwnerUsers[0],
			)
			addVmToOwnersTotals(totalsByOwner, &vm)
		}
	}
	switch parsedQuery.OutputType() {
	case url.OutputTypeHtml:
		tw.WriteRow("", "",
			"<b>TOTAL</b>",
			"",
			"",
			"",
			format.FormatBytes(allocatedMemoryInMiB<<20),
			format.FormatMilli(uint64(allocatedMilliCPUs)),
			strconv.Itoa(int(allocatedVirtualCPUs)),
			strconv.Itoa(int(numVolumes)),
			format.FormatBytes(allocatedVolumeSize),
			"",
		)
		capacity := s.manager.GetCapacity()
		tw.WriteRow("", "",
			"<b>CAPACITY</b>",
			"",
			"",
			"",
			format.FormatBytes(capacity.MemoryInMiB<<20),
			strconv.Itoa(int(capacity.NumCPUs)),
			"",
			"",
			format.FormatBytes(capacity.TotalVolumeBytes),
			"",
		)
		tw.WriteRow("", "",
			"<b>USAGE</b>",
			"",
			"",
			"",
			fmt.Sprintf("%d%%", allocatedMemoryInMiB*100/capacity.MemoryInMiB),
			fmt.Sprintf("%d%%", allocatedMilliCPUs/capacity.NumCPUs/10),
			"",
			"",
			fmt.Sprintf("%d%%",
				allocatedVolumeSize*100/capacity.TotalVolumeBytes),
			"",
		)
		tw.Close()
		fmt.Fprintln(writer, "<p>")
		fmt.Fprintln(writer, "VMs by primary owner:<br>")
		listVMsByPrimaryOwner(writer, totalsByOwner)
		fmt.Fprintln(writer, "</body>")
	}
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

func numVolumesTableEntry(vm proto.VmInfo) string {
	var comment string
	for _, volume := range vm.Volumes {
		if comment == "" && volume.Format != proto.VolumeFormatRaw {
			comment = `<font style="color:grey;font-size:12px"> (!RAW)</font>`
		}
	}
	return fmt.Sprintf("%d%s", len(vm.Volumes), comment)
}

func storageTotalTableEntry(vm proto.VmInfo) (uint64, string) {
	var storage uint64
	for _, volume := range vm.Volumes {
		storage += volume.Size
	}
	return storage, format.FormatBytes(storage)
}
