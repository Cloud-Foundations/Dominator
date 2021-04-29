package client

import (
	"io"

	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/imaginator"
)

func BuildImage(client *srpc.Client, request proto.BuildImageRequest,
	response *proto.BuildImageResponse, logWriter io.Writer) error {
	return buildImage(client, request, response, logWriter)
}

func GetDirectedGraph(client *srpc.Client,
	request proto.GetDirectedGraphRequest) ([]byte, error) {
	return getDirectedGraph(client, request)
}
