package topology

import (
	"path/filepath"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/repowatch"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func watch(topologyRepository, localRepositoryDir, topologyDir string,
	checkInterval time.Duration,
	logger log.DebugLogger) (<-chan *Topology, error) {
	directoryChannel, err := repowatch.Watch(topologyRepository,
		localRepositoryDir, checkInterval, "fleet-manager/topology-watcher",
		logger)
	if err != nil {
		return nil, err
	}
	topologyChannel := make(chan *Topology, 1)
	go handleNotifications(directoryChannel, topologyChannel, topologyDir,
		logger)
	return topologyChannel, nil
}

func handleNotifications(directoryChannel <-chan string,
	topologyChannel chan<- *Topology, topologyDir string,
	logger log.DebugLogger) {
	var prevTopology *Topology
	for dir := range directoryChannel {
		params := Params{
			Logger:      logger,
			TopologyDir: filepath.Join(dir, topologyDir),
		}
		if topology, err := load(params); err != nil {
			logger.Println(err)
		} else if prevTopology.equal(topology) {
			logger.Debugln(1, "Ignoring unchanged configuration")
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
