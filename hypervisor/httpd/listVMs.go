package httpd

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/html"
	"github.com/Cloud-Foundations/Dominator/lib/url"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

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
				fmt.Sprintf("%g", float64(vm.MilliCPUs)*1e-3),
				vCPUs,
				numVolumesTableEntry(vm),
				volumeString,
				vm.OwnerUsers[0],
			)
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
			fmt.Sprintf("%g", float64(allocatedMilliCPUs)*1e-3),
			strconv.Itoa(int(allocatedVirtualCPUs)),
			strconv.Itoa(int(numVolumes)),
			format.FormatBytes(allocatedVolumeSize),
			"",
		)
		tw.Close()
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
