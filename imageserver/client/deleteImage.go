package client

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func deleteImage(client srpc.ClientI, name string) error {
	request := imageserver.DeleteImageRequest{name}
	var reply imageserver.DeleteImageResponse
	return client.RequestReply("ImageServer.DeleteImage", request, &reply)
}
