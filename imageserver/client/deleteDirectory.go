package client

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func deleteDirectory(client srpc.ClientI, name string) error {
	request := imageserver.DeleteDirectoryRequest{name}
	var reply imageserver.DeleteDirectoryResponse
	err := client.RequestReply("ImageServer.DeleteDirectory", request,
		&reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}
