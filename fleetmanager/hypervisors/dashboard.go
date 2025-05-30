package hypervisors

import (
	"fmt"
	"io"
	"strings"
)

func (m *Manager) writeHtml(writer io.Writer) {
	t, err := m.getTopology()
	if err != nil {
		fmt.Fprintln(writer, err, "<br>")
		return
	}
	if *manageHypervisors {
		fmt.Fprintln(writer,
			`Hypervisors <font color="green">are</font> being managed by this instance<br>`)
	} else {
		fmt.Fprintln(writer,
			`<font color="grey">Hypervisors are not being managed by this instance</font><br>`)
	}
	numMachines := t.GetNumMachines()
	var numConnected, numDisabled, numOff, numOK uint
	m.mutex.RLock()
	for _, hypervisor := range m.hypervisors {
		switch hypervisor.probeStatus {
		case probeStatusConnected:
			numConnected++
			if hypervisor.disabled {
				numDisabled++
			}
			switch hypervisor.healthStatus {
			case "", "healthy":
				numOK++
			}
		case probeStatusOff:
			numOff++
		}
	}
	numVMs := uint(len(m.vms))
	m.mutex.RUnlock()
	writeCountLinksHT(writer, "Number of hypervisors known",
		"listHypervisors", numMachines)
	writeCountLinksHT(writer, "Number of hypervisors powered off",
		"listHypervisors?state=off", numOff)
	writeCountLinksHT(writer, "Number of hypervisors connected",
		"listHypervisors?state=connected", numConnected)
	writeCountLinksHT(writer, "Number of hypervisors disabled",
		"listHypervisors?state=disabled", numDisabled)
	writeCountLinksHT(writer, "Number of hypervisors OK",
		"listHypervisors?state=OK", numOK)
	writeCountLinksHTJ(writer, "Number of VMs known",
		"listVMs", numVMs)
	writeLinksHTJ(writer, "VMs by primary owner",
		"listVMsByPrimaryOwner", numVMs)
	fmt.Fprint(writer,
		`Hypervisor locations: <a href="listLocations?status=all">all</a>`)
	fmt.Fprint(writer,
		` (<a href="listLocations?output=text&status=all">text</a>)`)
	fmt.Fprint(writer,
		`, <a href="listLocations?status=any">any</a>`)
	fmt.Fprint(writer,
		` (<a href="listLocations?output=text&status=any">text</a>)`)
	fmt.Fprintln(writer,
		`, <a href="listLocations?status=healthy">healthy</a>`)
	fmt.Fprintln(writer,
		` (<a href="listLocations?output=text&status=healthy">text</a>)<br>`)
}

func writeCountLinksHT(writer io.Writer, text, path string, count uint) {
	if count < 1 {
		return
	}
	var separator string
	if strings.Contains(path, "?") {
		separator = "&"
	} else {
		separator = "?"
	}
	fmt.Fprintf(writer, "%s: <a href=\"%s\">%d</a>", text, path, count)
	fmt.Fprintf(writer, " (<a href=\"%s%soutput=text\">text</a>)",
		path, separator)
	fmt.Fprintf(writer, " (<a href=\"%s%soutput=json\">JSON</a>)<br>\n",
		path, separator)
}

func writeCountLinksHTJ(writer io.Writer, text, path string, count uint) {
	if count < 1 {
		return
	}
	fmt.Fprintf(writer,
		"%s: <a href=\"%s\">%d</a> (<a href=\"%s?output=text\">text</a>, <a href=\"%s?output=json\">JSON</a>)<br>\n",
		text, path, count, path, path)
}

func writeLinksHTJ(writer io.Writer, text, path string, count uint) {
	if count < 1 {
		return
	}
	fmt.Fprintf(writer,
		"%s: <a href=\"%s\">HTML</a>, <a href=\"%s?output=text\">text</a>, <a href=\"%s?output=json\">JSON</a><br>\n",
		text, path, path, path)
}
