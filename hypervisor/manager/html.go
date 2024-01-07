package manager

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/meminfo"
)

func (m *Manager) writeHtml(writer io.Writer) {
	if m.disabled {
		fmt.Fprintln(writer,
			`Hypervisor is <font color="red">disabled</font><p>`)
	}
	if wrote, _ := m.lockWatcher.WriteHtml(writer, ""); wrote {
		fmt.Fprintln(writer, "<br>")
	}
	summary := m.getSummary()
	if age := time.Since(summary.updatedAt); age > time.Second {
		fmt.Fprintf(writer,
			"<font color=\"salmon\">Dashboard data are %s old</font><p>\n",
			format.Duration(age))
	}
	writeCountLinks(writer, "Number of VMs known", "listVMs?state=",
		summary.numRunning+summary.numStopped)
	writeCountLinks(writer, "Number of VMs running", "listVMs?state=running",
		summary.numRunning)
	writeCountLinks(writer, "Number of VMs stopped", "listVMs?state=stopped",
		summary.numStopped)
	fmt.Fprintln(writer, "<br>")
	fmt.Fprintf(writer,
		"Available addresses: <a href=\"listAvailableAddresses\">%d</a><br>\n",
		summary.numFreeAddresses)
	fmt.Fprintf(writer,
		"Registered addresses: <a href=\"listRegisteredAddresses\">%d</a><br>\n",
		summary.numRegisteredAddresses)
	fmt.Fprintf(writer, "Available CPU: %s<br>\n",
		format.FormatMilli(uint64(summary.availableMilliCPU)))
	if memInfo, err := meminfo.GetMemInfo(); err != nil {
		fmt.Fprintf(writer, "Error getting available RAM: %s<br>\n", err)
	} else {
		fmt.Fprintf(writer, "Available RAM: real: %s, unallocated: %s<br>\n",
			format.FormatBytes(memInfo.Available),
			format.FormatBytes(summary.memUnallocated<<20))
	}
	sort.Strings(summary.ownerGroups)
	sort.Strings(summary.ownerUsers)
	if len(summary.ownerGroups) > 0 {
		fmt.Fprintf(writer, "Owner groups: %s<br>\n",
			strings.Join(summary.ownerGroups, " "))
	}
	if len(summary.ownerUsers) > 0 {
		fmt.Fprintf(writer, "Owner users: %s<br>\n",
			strings.Join(summary.ownerUsers, " "))
	}
	if m.serialNumber != "" {
		fmt.Fprintf(writer, "Serial number: \"%s\"<br>\n", m.serialNumber)
	}
	fmt.Fprintf(writer,
		"Number of subnets: <a href=\"listSubnets\">%d</a><br>\n",
		summary.numSubnets)
	fmt.Fprintf(writer,
		"Volume directories: <a href=\"showVolumeDirectories\">%d</a>",
		len(m.volumeInfos))
	fmt.Fprintln(writer, " <a href=\"listVolumeDirectories\">(text)</a><br>")
	if m.objectCache == nil {
		fmt.Fprintln(writer, "No object cache<br>")
	} else {
		m.objectCache.WriteHtml(writer)
	}
}

func writeCountLinks(writer io.Writer, text, path string, count uint) {
	fmt.Fprintf(writer,
		"%s: <a href=\"%s\">%d</a> (<a href=\"%s&output=text\">text</a>)<br>\n",
		text, path, count, path)
}
