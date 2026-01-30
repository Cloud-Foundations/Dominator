package manager

import (
	"github.com/Cloud-Foundations/Dominator/lib/meminfo"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (m *Manager) getCapacity() (proto.GetCapacityResponse, error) {
	memInfo, err := meminfo.GetMemInfo()
	if err != nil {
		return proto.GetCapacityResponse{}, err
	}
	return proto.GetCapacityResponse{
		AvailableMemoryInMiB: memInfo.Available >> 20,
		MemoryInMiB:          m.memTotalInMiB,
		NumCPUs:              m.numCPUs,
		TotalVolumeBytes:     m.totalVolumeBytes,
	}, nil
}
