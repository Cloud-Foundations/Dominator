package hypervisors

import (
	"bufio"
	"fmt"
	"net/http"

	"github.com/Cloud-Foundations/Dominator/fleetmanager/topology"
	"github.com/Cloud-Foundations/Dominator/lib/url"
)

func (m *Manager) listLocations(dirname string) ([]string, error) {
	topo, err := m.getTopology()
	if err != nil {
		return nil, err
	}
	directory, err := topo.FindDirectory(dirname)
	if err != nil {
		return nil, err
	}
	var locations []string
	directory.Walk(func(directory *topology.Directory) error {
		for _, machine := range directory.Machines {
			hypervisor, err := m.getLockedHypervisor(machine.Hostname, false)
			if err != nil {
				continue
			}
			if hypervisor.probeStatus == probeStatusConnected &&
				(hypervisor.healthStatus == "" ||
					hypervisor.healthStatus == "healthy") {
				locations = append(locations, directory.GetPath())
				hypervisor.mutex.RUnlock()
				return nil
			}
			hypervisor.mutex.RUnlock()
		}
		return nil
	})
	return locations, nil
}

func (m *Manager) listLocationsHandler(w http.ResponseWriter,
	req *http.Request) {
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	parsedQuery := url.ParseQuery(req.URL)
	locations, err := m.listLocations("")
	if err != nil {
		fmt.Fprintln(writer, err)
		return
	}
	switch parsedQuery.OutputType() {
	case url.OutputTypeHtml:
		fmt.Fprintf(writer, "<title>List of Hypervisor Locations</title>\n")
		fmt.Fprintln(writer, "<body>")
		for _, location := range locations {
			fmt.Fprintf(writer,
				"<a href=\"listHypervisors?location=%s\">%s</a><br>\n",
				location, location)
		}
		fmt.Fprintln(writer, "</body>")
	case url.OutputTypeText:
		for _, location := range locations {
			fmt.Fprintln(writer, location)
		}
	}
}
