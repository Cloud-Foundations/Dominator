package hypervisors

import (
	"bufio"
	"fmt"
	"net"
	"net/http"

	"github.com/Cloud-Foundations/Dominator/lib/json"
	fm_proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (m *Manager) getHypervisorForRequest(w http.ResponseWriter,
	req *http.Request) *hypervisorType {
	if hostname := req.FormValue("hostname"); hostname != "" {
		h, err := m.getLockedHypervisor(hostname, false)
		if err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			return nil
		}
		return h
	}
	ipAddr := req.FormValue("ip")
	if ipAddr == "" {
		var err error
		ipAddr, _, err = net.SplitHostPort(req.RemoteAddr)
		if err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return nil
		}
	}
	h, err := m.getLockedHypervisorByIP(ipAddr)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		return nil
	}
	return h
}

func (m *Manager) tftpdataConfigHandler(w http.ResponseWriter,
	req *http.Request) {
	topo, err := m.getTopology()
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h := m.getHypervisorForRequest(w, req)
	if h == nil {
		return
	}
	defer h.mutex.RUnlock()
	subnets, _ := topo.GetSubnetsForMachine(h.Hostname)
	hSubnets := make([]*hyper_proto.Subnet, 0, len(subnets))
	for _, subnet := range subnets {
		hSubnets = append(hSubnets, &subnet.Subnet)
	}
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	w.Header().Set("Content-Type", "application/json")
	json.WriteWithIndent(writer, "    ", fm_proto.GetMachineInfoResponse{
		Location: h.location,
		Machine:  h.Machine,
		Subnets:  hSubnets,
	})
}

func (m *Manager) tftpdataImageNameHandler(w http.ResponseWriter,
	req *http.Request) {
	h := m.getHypervisorForRequest(w, req)
	if h == nil {
		return
	}
	defer h.mutex.RUnlock()
	imageName := h.Hypervisor.Tags["RequiredImage"]
	if override := h.localTags["RequiredImage"]; override != "" {
		imageName = override
	}
	if imageName == "" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	fmt.Fprintln(w, imageName)
}
