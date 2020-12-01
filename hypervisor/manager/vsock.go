package manager

import (
	"net"

	"github.com/Cloud-Foundations/Dominator/lib/net/vsock"
)

func (m *Manager) checkVsockets() error {
	if cid, err := vsock.GetContextID(); err != nil {
		return nil
	} else if cid != 2 {
		m.Logger.Printf("detected VSOCK CID=%d, not enabling\n", cid)
	} else {
		m.vsocketsEnabled = true
		m.Logger.Println("VSOCK enabled")
	}
	return nil
}

func (m *Manager) getVmCID(ipAddr net.IP) (uint32, error) {
	if !m.vsocketsEnabled {
		return 0, nil
	}
	if ip4 := ipAddr.To4(); ip4 == nil {
		return 0, nil
	} else {
		return uint32(ip4[0])<<24 |
				uint32(ip4[1])<<16 |
				uint32(ip4[2])<<8 |
				uint32(ip4[3]),
			nil
	}
}
