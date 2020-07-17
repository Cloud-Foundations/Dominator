package httpd

import (
	"bufio"
	"fmt"
	"net"
	"net/http"

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
			"Name(tag)", "State", "RAM", "CPU", "Num Volumes", "Storage",
			"Primary Owner")
	}
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
			var background string
			if vm.Uncommitted {
				background = "yellow"
			}
			tw.WriteRow("", background,
				fmt.Sprintf("<a href=\"showVM?%s\">%s</a>", ipAddr, ipAddr),
				vm.Address.MacAddress,
				vm.Tags["Name"],
				vm.State.String(),
				format.FormatBytes(vm.MemoryInMiB<<20),
				fmt.Sprintf("%g", float64(vm.MilliCPUs)*1e-3),
				numVolumesTableEntry(vm),
				storageTotalTableEntry(vm),
				vm.OwnerUsers[0],
			)
		}
	}
	switch parsedQuery.OutputType() {
	case url.OutputTypeHtml:
		fmt.Fprintln(writer, "</table>")
		fmt.Fprintln(writer, "</body>")
	}
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

func storageTotalTableEntry(vm proto.VmInfo) string {
	var storage uint64
	for _, volume := range vm.Volumes {
		storage += volume.Size
	}
	return format.FormatBytes(storage)
}
