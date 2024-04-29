package rpcd

import (
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) WatchDhcp(conn *srpc.Conn) error {
	var request proto.WatchDhcpRequest
	if err := conn.Decode(&request); err != nil {
		return err
	}
	closeChannel := conn.GetCloseNotifier()
	packetChannel := t.dhcpServer.MakePacketWatchChannel()
	defer t.dhcpServer.ClosePacketWatchChannel(packetChannel)
	flushTimer := time.NewTimer(flushDelay)
	var numToFlush uint
	maxPackets := request.MaxPackets
	for count := uint64(0); maxPackets < 1 || count < maxPackets; {
		select {
		case packet, ok := <-packetChannel:
			if !ok {
				msg := proto.WatchDhcpResponse{
					Error: "receiver not keeping up with DHCP packets",
				}
				return conn.Encode(msg)
			}
			if request.Interface != "" &&
				packet.Interface != request.Interface {
				continue
			}
			if err := conn.Encode(packet); err != nil {
				t.logger.Printf("error sending packet: %s\n", err)
				return err
			}
			if packet.Error != "" {
				return nil
			}
			count++
			numToFlush++
			flushTimer.Reset(flushDelay)
		case <-flushTimer.C:
			if numToFlush > 1 {
				t.logger.Debugf(0, "flushing %d packets\n", numToFlush)
			}
			numToFlush = 0
			if err := conn.Flush(); err != nil {
				t.logger.Printf("error flushing packet(s): %s\n", err)
				return err
			}
		case err := <-closeChannel:
			if err == nil {
				t.logger.Debugf(0, "packet client disconnected: %s\n",
					conn.RemoteAddr())
				return nil
			}
			t.logger.Println(err)
			return err
		}
	}
	return nil
}
