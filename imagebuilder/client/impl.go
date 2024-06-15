package client

import (
	"fmt"
	"io"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/imaginator"
)

func buildImage(client *srpc.Client, request proto.BuildImageRequest,
	response *proto.BuildImageResponse, logWriter io.Writer) error {
	conn, err := client.Call("Imaginator.BuildImage")
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.Encode(request); err != nil {
		return err
	}
	if err := conn.Flush(); err != nil {
		return err
	}
	str, err := conn.ReadString('\n')
	if err != nil {
		return err
	}
	if str != "\n" {
		return errors.New(str[:len(str)-1])
	}
	for {
		var reply proto.BuildImageResponse
		if err := conn.Decode(&reply); err != nil {
			return fmt.Errorf("error reading reply: %s", err)
		}
		logWriter.Write(reply.BuildLog)
		reply.BuildLog = nil
		if err := errors.New(reply.ErrorString); err != nil {
			*response = reply
			return err
		}
		if reply.Image != nil || reply.ImageName != "" {
			*response = reply
			return nil
		}
	}
}

func disableAutoBuilds(client *srpc.Client, disableFor time.Duration) (
	time.Time, error) {
	var reply proto.DisableAutoBuildsResponse
	err := client.RequestReply("Imaginator.DisableAutoBuilds",
		proto.DisableAutoBuildsRequest{
			DisableFor: disableFor,
		}, &reply)
	if err != nil {
		return time.Time{}, err
	}
	err = errors.New(reply.Error)
	if err != nil {
		return time.Time{}, err
	}
	return reply.DisabledUntil, nil
}

func disableBuildRequests(client *srpc.Client, disableFor time.Duration) (
	time.Time, error) {
	var reply proto.DisableBuildRequestsResponse
	err := client.RequestReply("Imaginator.DisableBuildRequests",
		proto.DisableBuildRequestsRequest{
			DisableFor: disableFor,
		}, &reply)
	if err != nil {
		return time.Time{}, err
	}
	err = errors.New(reply.Error)
	if err != nil {
		return time.Time{}, err
	}
	return reply.DisabledUntil, nil
}

func getDependencies(client *srpc.Client,
	request proto.GetDependenciesRequest) (
	proto.GetDependenciesResult, error) {
	var zero proto.GetDependenciesResult
	var reply proto.GetDependenciesResponse
	err := client.RequestReply("Imaginator.GetDependencies", request, &reply)
	if err != nil {
		return zero, err
	}
	if reply.Error != "" {
		return zero, errors.New(reply.Error)
	}
	return reply.GetDependenciesResult, nil
}

func getDirectedGraph(client *srpc.Client,
	request proto.GetDirectedGraphRequest) (
	proto.GetDirectedGraphResult, error) {
	var zero proto.GetDirectedGraphResult
	var reply proto.GetDirectedGraphResponse
	err := client.RequestReply("Imaginator.GetDirectedGraph", request, &reply)
	if err != nil {
		return zero, err
	}
	if reply.Error != "" {
		return zero, errors.New(reply.Error)
	}
	return reply.GetDirectedGraphResult, nil
}

func replaceIdleSlaves(client *srpc.Client, immediateGetNew bool) error {
	var reply proto.ReplaceIdleSlavesResponse
	err := client.RequestReply("Imaginator.ReplaceIdleSlaves",
		proto.ReplaceIdleSlavesRequest{
			ImmediateGetNew: immediateGetNew,
		}, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}
