package rpcd

import (
	"fmt"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
)

const flushDelay = time.Millisecond * 10

func (t *srpcType) GetUpdates(conn *srpc.Conn) error {
	var request proto.GetUpdatesRequest
	if err := conn.Decode(&request); err != nil {
		return err
	}
	closeChannel := conn.GetCloseNotifier()
	updateChannel := t.hypervisorsManager.MakeUpdateChannel(request)
	defer t.hypervisorsManager.CloseUpdateChannel(updateChannel)
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
			if err := conn.Encode(update); err != nil {
				return fmt.Errorf("error sending update: %s", err)
			}
			if update.Error != "" {
				return nil
			}
			count++
			numToFlush++
			flushTimer.Reset(flushDelay)
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
				t.logger.Debugf(0, "update client disconnected: %s\n",
					conn.RemoteAddr())
				return nil
			}
			return err
		}
	}
	return nil
}
