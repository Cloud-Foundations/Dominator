package hypervisors

import (
	"bufio"
	"fmt"
	"net/http"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/url"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

// getVmInfoAndHypervisor returns VM info if it exists (else nil) and the
// hostname of its Hypervisor if the Hypervisor is OK (else it returns an empty
// string).
func (m *Manager) getVmInfoAndHypervisor(vmIpAddr string) (
	*hyper_proto.VmInfo, string) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	vm := m.vms[vmIpAddr]
	if vm == nil {
		return nil, ""
	}
	if vm.hypervisor.probeStatus == probeStatusConnected {
		return &vm.VmInfo, vm.hypervisor.machine.Hostname
	}
	return &vm.VmInfo, ""
}

func (m *Manager) showVmHandler(w http.ResponseWriter,
	req *http.Request) {
	parsedQuery := url.ParseQuery(req.URL)
	if len(parsedQuery.Flags) != 1 {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var ipAddr string
	for name := range parsedQuery.Flags {
		ipAddr = name
	}
	vmInfo, hypervisorHostname := m.getVmInfoAndHypervisor(ipAddr)
	if vmInfo == nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if hypervisorHostname != "" {
		http.Redirect(w, req,
			fmt.Sprintf("http://%s:%d/showVM?%s",
				hypervisorHostname, constants.HypervisorPortNumber, ipAddr),
			http.StatusFound)
	}
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	json.WriteWithIndent(writer, "    ", vmInfo)
}
