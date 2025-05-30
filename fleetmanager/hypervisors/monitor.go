package hypervisors

import (
	"flag"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

var (
	hypervisorProbeTimeout = flag.Duration("hypervisorProbeTimeout",
		time.Second*5, "time after which a probe is sent to a quiet Hypervisor")
	hypervisorResponseTimeout = flag.Duration("hypervisorResponseTimeout",
		time.Second*19,
		"time after which a Hypervisor is marked as unresponsive")
)

func (h *hypervisorType) monitorLoop(client *srpc.Client, conn *srpc.Conn,
	closeClientChannel <-chan struct{}) {
	pingDeferChannel := make(chan struct{})
	defer close(pingDeferChannel)
	go h.pingLoop(conn, pingDeferChannel)
	lastReceiveTime := time.Now()
	for {
		timeout := *hypervisorResponseTimeout - time.Since(lastReceiveTime)
		if timeout <= 0 {
			timeout = time.Millisecond
		}
		timer := time.NewTimer(timeout)
		select {
		case <-closeClientChannel:
			client.Close()
			return
		case _, ok := <-h.receiveChannel:
			if !timer.Stop() {
				<-timer.C
			}
			if !ok {
				return
			}
			select {
			case pingDeferChannel <- struct{}{}:
			default:
			}
			lastReceiveTime = time.Now()
			h.mutex.Lock()
			h.lastConnectedTime = lastReceiveTime
			h.probeStatus = probeStatusConnected
			h.mutex.Unlock()
		case <-timer.C:
			h.mutex.Lock()
			h.probeStatus = probeStatusUnreachable
			h.mutex.Unlock()
			h.logger.Debugln(0, "shutting down unresponsive client")
			client.Close()
			return
		}
	}
}

func (h *hypervisorType) pingLoop(conn *srpc.Conn,
	pingDeferChannel <-chan struct{}) {
	pingsSinceLastDefer := 0
	for {
		timer := time.NewTimer(*hypervisorProbeTimeout)
		select {
		case _, ok := <-pingDeferChannel:
			if !timer.Stop() {
				<-timer.C
			}
			if !ok {
				return
			}
			timer.Reset(*hypervisorProbeTimeout)
			h.mutex.Lock()
			h.probeStatus = probeStatusConnected
			h.mutex.Unlock()
			pingsSinceLastDefer = 0
		case <-timer.C:
			pingsSinceLastDefer++
			if pingsSinceLastDefer > 1 {
				h.logger.Debugf(0, "sending ping #%d since last activity\n",
					pingsSinceLastDefer)
			} else {
				h.logger.Debugln(1, "sending first ping since last activity")
			}
			err := conn.Encode(hyper_proto.GetUpdatesRequest{})
			if err != nil {
				h.logger.Printf("error sending ping: %s\n", err)
			} else {
				if err := conn.Flush(); err != nil {
					h.logger.Printf("error flushing ping: %s\n", err)
				}
			}
			timer.Reset(*hypervisorProbeTimeout)
		}
	}
}
