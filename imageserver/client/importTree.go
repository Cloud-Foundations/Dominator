package client

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func importTree(client srpc.ClientI, request proto.ImportTreeRequest) (
	proto.ImportTreeResponse, error) {
	var reply proto.ImportTreeResponse
	err := client.RequestReply("ImageServer.ImportTree", request, &reply)
	if err == nil {
		err = errors.New(reply.Error)
	}
	if err != nil {
		return proto.ImportTreeResponse{}, err
	}
	return reply, nil
}
