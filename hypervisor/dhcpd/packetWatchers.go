package dhcpd

import (
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

// This will grab and release the lock.
func (s *DhcpServer) closePacketWatchChannel(
	channel <-chan proto.WatchDhcpResponse) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.packetWatchers, channel)
}

// This will grab and release the lock.
func (s *DhcpServer) makePacketWatchChannel() <-chan proto.WatchDhcpResponse {
	channel := make(chan proto.WatchDhcpResponse, 16)
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.packetWatchers[channel] = channel
	return channel
}

// This will grab and release the lock.
func (s *DhcpServer) sendPacket(interfaceName string, inputPacket []byte) {
	packet := make([]byte, len(inputPacket))
	copy(packet, inputPacket)
	msg := proto.WatchDhcpResponse{
		Interface: interfaceName,
		Packet:    packet,
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for rChannel, sChannel := range s.packetWatchers {
		select {
		case sChannel <- msg:
		default:
			delete(s.packetWatchers, rChannel)
			close(sChannel)
		}
	}
}
