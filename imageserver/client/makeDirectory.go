package client

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func makeDirectory(client srpc.ClientI, dirname string, all bool) error {
	request := imageserver.MakeDirectoryRequest{
		DirectoryName: dirname,
		MakeAll:       all,
	}
	var reply imageserver.MakeDirectoryResponse
	return client.RequestReply("ImageServer.MakeDirectory", request, &reply)
}
