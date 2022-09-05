package topology

import (
	"path/filepath"

	"github.com/Cloud-Foundations/Dominator/lib/repowatch"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func watch(params WatchParams) (<-chan *Topology, error) {
	if params.MetricsDirectory == "" {
		params.MetricsDirectory = "fleet-manager/topology-watcher"
	}
	directoryChannel, err := repowatch.Watch(params.TopologyRepository,
		params.LocalRepositoryDir, params.CheckInterval,
		params.MetricsDirectory, params.Logger)
	if err != nil {
		return nil, err
	}
	topologyChannel := make(chan *Topology, 1)
	go handleNotifications(directoryChannel, topologyChannel, params.Params)
	return topologyChannel, nil
}

func handleNotifications(directoryChannel <-chan string,
	topologyChannel chan<- *Topology, params Params) {
	var prevTopology *Topology
	for dir := range directoryChannel {
		loadParams := params
		loadParams.TopologyDir = filepath.Join(dir, params.TopologyDir)
		if topology, err := load(loadParams); err != nil {
			params.Logger.Println(err)
		} else if prevTopology.equal(topology) {
			params.Logger.Debugln(1, "Ignoring unchanged configuration")
		} else {
			topologyChannel <- topology
			prevTopology = topology
		}
	}
}

func (subnet *Subnet) shrink() {
	subnet.Subnet.Shrink()
	subnet.FirstAutoIP = hyper_proto.ShrinkIP(subnet.FirstAutoIP)
	subnet.LastAutoIP = hyper_proto.ShrinkIP(subnet.LastAutoIP)
	for index, ip := range subnet.ReservedIPs {
		if len(ip) == 16 {
			ip = ip.To4()
			if ip != nil {
				subnet.ReservedIPs[index] = ip
			}
		}
	}
}
