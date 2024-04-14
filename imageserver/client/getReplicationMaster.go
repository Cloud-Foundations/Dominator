package client

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func getReplicationMaster(client srpc.ClientI) (string, error) {
	request := imageserver.GetReplicationMasterRequest{}
	var reply imageserver.GetReplicationMasterResponse
	err := client.RequestReply("ImageServer.GetReplicationMaster", request,
		&reply)
	if err != nil {
		return "", err
	}
	if err := errors.New(reply.Error); err != nil {
		return "", err
	}
	return reply.ReplicationMaster, nil
}
