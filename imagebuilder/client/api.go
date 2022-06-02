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

func GetDependencies(client *srpc.Client,
	request proto.GetDependenciesRequest) (
	proto.GetDependenciesResult, error) {
	return getDependencies(client, request)
}

func GetDirectedGraph(client *srpc.Client,
	request proto.GetDirectedGraphRequest) (
	proto.GetDirectedGraphResult, error) {
	return getDirectedGraph(client, request)
}
