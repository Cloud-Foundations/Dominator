package hypervisors

import (
	"fmt"
	"math/rand"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/net/util"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

const powerOff = "Power is off"

var (
	myIP    net.IP
	wolConn *net.UDPConn
)

func extractSerialNumber(input string) string {
	serial := strings.TrimSpace(input)
	// Ignore some common bogus serial numbers.
	switch serial {
	case "0123456789":
		serial = ""
	case "System Serial Number":
		serial = ""
	case "To be filled by O.E.M.":
		serial = ""
	}
	return serial
}

func (m *Manager) ipmiGetSlot() {
	m.ipmiLimiter <- struct{}{}
}

func (m *Manager) ipmiReleaseSlot() {
	<-m.ipmiLimiter
}

func (m *Manager) powerOnMachine(hostname string,
	authInfo *srpc.AuthInformation) error {
	h, err := m.getLockedHypervisor(hostname, false)
	if err != nil {
		return err
	}
	defer h.mutex.RUnlock()
	if err := h.checkAuth(authInfo); err != nil {
		return err
	}
	var ipmiHostname string
	if len(h.machine.IPMI.HostIpAddress) > 0 {
		ipmiHostname = h.machine.IPMI.HostIpAddress.String()
	} else if h.machine.IPMI.Hostname != "" {
		ipmiHostname = h.machine.IPMI.Hostname
	} else if sentWakeOnLan, err := m.wakeOnLan(h); err != nil {
		return err
	} else if sentWakeOnLan {
		return nil
	} else {
		return fmt.Errorf("no IPMI address for: %s", hostname)
	}
	cmd := exec.Command("ipmitool", "-f", m.ipmiPasswordFile,
		"-H", ipmiHostname, "-I", "lanplus", "-U", m.ipmiUsername,
		"chassis", "power", "on")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s", err, string(output))
	}
	return nil
}

// probeSerialNumber will start a delayed background IPMI probe of the serial
// number if not discovered otherwise.
func (m *Manager) probeSerialNumber(h *hypervisorType) {
	if h.serialNumber != "" {
		return
	}
	if m.ipmiPasswordFile == "" || m.ipmiUsername == "" {
		return
	}
	var ipmiHostname string
	if len(h.machine.IPMI.HostIpAddress) > 0 {
		ipmiHostname = h.machine.IPMI.HostIpAddress.String()
	} else if h.machine.IPMI.Hostname != "" {
		ipmiHostname = h.machine.IPMI.Hostname
	} else {
		return
	}
	// Run the rest in the background.
	go func() {
		time.Sleep(5 * time.Second)
		if h.isDeleteScheduled() {
			return
		}
		if h.getSerialNumber() != "" {
			return
		}
		serialNumber := m.readSerialNumber(ipmiHostname)
		if h.isDeleteScheduled() {
			return
		}
		if serialNumber == "" {
			return
		}
		if h.getSerialNumber() != "" {
			return
		}
		h.mutex.Lock()
		if h.serialNumber != "" {
			h.mutex.Unlock()
			return
		}
		h.serialNumber = serialNumber
		h.mutex.Unlock()
		err := m.storer.WriteMachineSerialNumber(h.machine.HostIpAddress,
			serialNumber)
		if err != nil {
			h.logger.Println(err)
		} else {
			h.mutex.Lock()
			h.cachedSerialNumber = serialNumber
			h.mutex.Unlock()
		}
	}()
}

func (m *Manager) probeUnreachable(h *hypervisorType) probeStatus {
	if m.ipmiPasswordFile == "" || m.ipmiUsername == "" {
		return probeStatusUnreachable
	}
	var ipmiHostname string
	if len(h.machine.IPMI.HostIpAddress) > 0 {
		ipmiHostname = h.machine.IPMI.HostIpAddress.String()
	} else if h.machine.IPMI.Hostname != "" {
		ipmiHostname = h.machine.IPMI.Hostname
	} else {
		return probeStatusUnreachable
	}
	h.mutex.RLock()
	previousProbeStatus := h.probeStatus
	h.mutex.RUnlock()
	mimimumProbeInterval := time.Second * time.Duration(30+rand.Intn(30))
	if previousProbeStatus == probeStatusOff &&
		time.Until(h.lastIpmiProbe.Add(mimimumProbeInterval)) > 0 {
		return probeStatusOff
	}
	cmd := exec.Command("ipmitool", "-f", m.ipmiPasswordFile,
		"-H", ipmiHostname, "-I", "lanplus", "-U", m.ipmiUsername,
		"chassis", "power", "status")
	h.lastIpmiProbe = time.Now()
	if output, err := cmd.Output(); err != nil {
		if previousProbeStatus == probeStatusOff {
			return probeStatusOff
		} else {
			return probeStatusUnreachable
		}
	} else if strings.Contains(string(output), powerOff) {
		return probeStatusOff
	}
	return probeStatusUnreachable
}

func (m *Manager) readSerialNumber(ipmiHostname string) string {
	m.ipmiGetSlot()
	cmd := exec.Command("ipmitool", "-f", m.ipmiPasswordFile,
		"-H", ipmiHostname, "-I", "lanplus", "-U", m.ipmiUsername,
		"fru", "print")
	output, err := cmd.Output()
	m.ipmiReleaseSlot()
	if err != nil {
		return ""
	}
	var boardSerial, productSerial string
	for _, line := range strings.Split(string(output), "\n") {
		splitLine := strings.Split(line, ":")
		if len(splitLine) != 2 {
			continue
		}
		switch strings.TrimSpace(splitLine[0]) {
		case "Board Serial":
			boardSerial = extractSerialNumber(splitLine[1])
		case "Product Serial":
			productSerial = extractSerialNumber(splitLine[1])
		}
	}
	if productSerial != "" {
		return productSerial
	}
	return boardSerial
}

func (m *Manager) wakeOnLan(h *hypervisorType) (bool, error) {
	if len(h.machine.HostMacAddress) < 1 {
		return false, nil
	}
	routeTable, err := util.GetRouteTable()
	if err != nil {
		return false, err
	}
	var routeEntry *util.RouteEntry
	for _, route := range routeTable.RouteEntries {
		if route.Flags&util.RouteFlagUp == 0 {
			continue
		}
		if route.Flags&util.RouteFlagGateway != 0 {
			continue
		}
		if h.machine.HostIpAddress.Mask(route.Mask).Equal(route.BaseAddr) {
			routeEntry = route
			break
		}
	}
	if routeEntry == nil {
		return false, nil
	}
	if wolConn == nil {
		myIP, err = util.GetMyIP()
		if err != nil {
			return false, err
		}
		wolConn, err = net.ListenUDP("udp", &net.UDPAddr{IP: myIP})
		if err != nil {
			return false, err
		}
	}
	packet := []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	for count := 0; count < 16; count++ {
		packet = append(packet, h.machine.HostMacAddress...)
	}
	remoteAddr := &net.UDPAddr{IP: routeEntry.BroadcastAddr, Port: 9}
	if _, err := wolConn.WriteToUDP(packet, remoteAddr); err != nil {
		return false, err
	}
	return true, nil
}
