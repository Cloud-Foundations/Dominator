package rpcd

import (
	"fmt"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
)

func (t *srpcType) GetAllocationUpdates(conn *srpc.Conn) error {
	var request proto.GetAllocationUpdatesRequest
	if err := conn.Decode(&request); err != nil {
		return err
	}
	var closeChannel <-chan error
	if request.MaxUpdates == 0 && request.UntilRequestId == "" {
		// Client wants updates forever, so get early notification of closure.
		// This poisons the client connection for any subsequent methods.
		closeChannel = conn.GetCloseNotifier()
	}
	updateQueue := t.allocationManager.GetUpdateQueue()
	updateChannel := updateQueue.Subscribe(request.Position)
	defer updateQueue.CloseSubscriber(updateChannel)
	flushTimer := time.NewTimer(flushDelay)
	var numToFlush uint
	maxUpdates := request.MaxUpdates
	for count := uint64(0); maxUpdates < 1 || count < maxUpdates; {
		select {
		case update, ok := <-updateChannel:
			if !ok {
				return fmt.Errorf(
					"error sending update to: %s for: %s: receiver not keeping up with updates",
					conn.RemoteAddr(), conn.Username())
			}
			entry := update.Value
			if !request.IncludeRequests {
				entry.Request = nil
			}
			updateMessage := proto.AllocationUpdate{
				AllocationUpdateEntry: entry,
				Position:              update.Position,
			}
			if err := conn.Encode(updateMessage); err != nil {
				return fmt.Errorf("error sending update: %s", err)
			}
			if request.UntilRequestId != "" &&
				request.UntilRequestId == update.Value.RequestId {
				return nil
			}
			count++
			numToFlush++
			if !flushTimer.Stop() {
				select {
				case <-flushTimer.C:
				default:
				}
			}
			if len(updateChannel) < 1 {
				flushTimer.Reset(flushDelay)
			}
		case <-flushTimer.C:
			if numToFlush > 1 {
				t.logger.Debugf(0, "flushing %d events\n", numToFlush)
			}
			numToFlush = 0
			if err := conn.Flush(); err != nil {
				return fmt.Errorf("error flushing update(s): %s", err)
			}
		case err := <-closeChannel:
			if err == nil {
				t.logger.Debugf(0,
					"allocation update client disconnected: %s\n",
					conn.RemoteAddr())
				return nil
			}
			return err
		}
	}
	return nil
}
