package hypervisors

import (
	"bufio"
	"fmt"
	"net"
	"net/http"

	"github.com/Cloud-Foundations/Dominator/lib/firmware"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	fm_proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func getFormSerialNumber(req *http.Request) string {
	serialNumber := req.FormValue("serial_number")
	if serialNumber == "" {
		return ""
	}
	return firmware.ExtractSerialNumber(serialNumber)
}

func (m *Manager) getHypervisorForRequest(w http.ResponseWriter,
	req *http.Request) *hypervisorType {
	if err := req.ParseForm(); err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return nil
	}
	if hostname := req.FormValue("hostname"); hostname != "" {
		h, err := m.getLockedHypervisor(hostname, false)
		if err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			m.logger.Debugf(0,
				"/tftpdata handler(%s): failed to find Hypervisor for hostname=%s\n",
				req.RemoteAddr, hostname)
			return nil
		}
		m.logger.Debugf(0,
			"/tftpdata handler(%s): got Hypervisor for hostname=%s\n",
			req.RemoteAddr, hostname)
		return h
	}
	if ipAddr := req.FormValue("ip"); ipAddr != "" {
		h, err := m.getLockedHypervisorByIP(ipAddr)
		if err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			m.logger.Debugf(0,
				"/tftpdata handler(%s): failed to find Hypervisor for IP=%s\n",
				req.RemoteAddr, ipAddr)
			return nil
		}
		m.logger.Debugf(0,
			"/tftpdata handler(%s): got Hypervisor for IP=%s (host: %s)\n",
			req.RemoteAddr, ipAddr, h.Hostname)
		return h
	}
	if serialNumber := getFormSerialNumber(req); serialNumber != "" {
		if h, err := m.getLockedHypervisorBySN(serialNumber); err == nil {
			m.logger.Debugf(0,
				"/tftpdata handler(%s): got Hypervisor for SN=%s (host: %s)\n",
				req.RemoteAddr, serialNumber, h.Hostname)
			return h
		}
	}
	ipAddr, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil
	}
	if h, err := m.getLockedHypervisorByIP(ipAddr); err == nil {
		m.logger.Debugf(0,
			"/tftpdata handler(%s): got Hypervisor by IP (host: %s)\n",
			req.RemoteAddr, h.Hostname)
		return h
	}
	for _, macAddr := range req.Form["mac"] {
		if h, err := m.getLockedHypervisorByHW(macAddr); err == nil {
			m.logger.Debugf(0,
				"/tftpdata handler(%s): got Hypervisor for MAC=%s (host: %s)\n",
				req.RemoteAddr, macAddr, h.Hostname)
			return h
		}
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	m.logger.Debugf(0, "/tftpdata handler(%s): failed to find Hypervisor\n",
		req.RemoteAddr)
	return nil
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

func (m *Manager) tftpdataStorageLayoutHandler(w http.ResponseWriter,
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
	installConfig, _ := topo.GetInstallConfigForMachine(h.Hostname)
	if installConfig == nil || installConfig.StorageLayout == nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	w.Header().Set("Content-Type", "application/json")
	json.WriteWithIndent(writer, "    ", installConfig.StorageLayout)
}
