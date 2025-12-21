package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/dominator"
)

func (t *rpcType) FastUpdate(conn *srpc.Conn,
	decoder srpc.Decoder, encoder srpc.Encoder) error {
	var request dominator.FastUpdateRequest
	if err := decoder.Decode(&request); err != nil {
		return err
	}
	var imageType string
	if request.UsePlannedImage {
		imageType = "Planned"
	} else {
		imageType = "Required"
	}
	if conn.Username() == "" {
		t.logger.Printf("FastUpdate(%s) with %sImage\n",
			request.Hostname, imageType)
	} else {
		t.logger.Printf("FastUpdate(%s) with %sImage: by %s\n",
			request.Hostname, imageType, conn.Username())
	}
	progressChannel, err := t.herd.FastUpdate(request,
		conn.GetAuthInformation())
	if err != nil {
		reply := dominator.FastUpdateResponse{Error: err.Error()}
		if err := encoder.Encode(reply); err != nil {
			return err
		}
		return nil
	}
	// The last message before progressChannel closes will contain the
	// completion information, so retain outside loop.
	var reply dominator.FastUpdateResponse
	for {
		progressMessage, ok := <-progressChannel
		if ok {
			reply.ProcessingTime = progressMessage.ProcessingTime
			reply.ProgressMessage = progressMessage.Message
			reply.QueueTime = progressMessage.QueueTime
			reply.RebootBlocked = progressMessage.RebootBlocked
			reply.Synced = progressMessage.Synced
		} else {
			reply.Final = true
			reply.ProgressMessage = ""
		}
		if err := encoder.Encode(reply); err != nil {
			return err
		}
		if len(progressChannel) < 1 {
			if err := conn.Flush(); err != nil {
				return err
			}
		}
		if !ok {
			return nil
		}
	}
}
