package client

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/dominator"
	subproto "github.com/Cloud-Foundations/Dominator/proto/sub"
)

func clearSafetyShutoff(client srpc.ClientI, subHostname string) error {
	request := proto.ClearSafetyShutoffRequest{Hostname: subHostname}
	var reply proto.ClearSafetyShutoffResponse
	return client.RequestReply("Dominator.ClearSafetyShutoff", request, &reply)
}

func configureSubs(client srpc.ClientI,
	configuration subproto.Configuration) error {
	request := proto.ConfigureSubsRequest(configuration)
	var reply proto.ConfigureSubsResponse
	return client.RequestReply("Dominator.ConfigureSubs", request, &reply)
}

func disableUpdates(client srpc.ClientI, reason string) error {
	if reason == "" {
		return errors.New("cannot disable updates: no reason given")
	}
	request := proto.DisableUpdatesRequest{Reason: reason}
	var reply proto.DisableUpdatesResponse
	return client.RequestReply("Dominator.DisableUpdates", request, &reply)
}

func enableUpdates(client srpc.ClientI, reason string) error {
	if reason == "" {
		return errors.New("cannot enable updates: no reason given")
	}
	request := proto.EnableUpdatesRequest{Reason: reason}
	var reply proto.EnableUpdatesResponse
	return client.RequestReply("Dominator.EnableUpdates", request, &reply)
}

func fastUpdate(client srpc.ClientI, request proto.FastUpdateRequest,
	logger log.DebugLogger) (proto.FastUpdateResponse, error) {
	conn, err := client.Call("Dominator.FastUpdate")
	if err != nil {
		return proto.FastUpdateResponse{}, err
	}
	defer conn.Close()
	if err := conn.Encode(request); err != nil {
		return proto.FastUpdateResponse{}, err
	}
	if err := conn.Flush(); err != nil {
		return proto.FastUpdateResponse{}, err
	}
	for {
		var reply proto.FastUpdateResponse
		if err := conn.Decode(&reply); err != nil {
			return proto.FastUpdateResponse{},
				fmt.Errorf("error decoding: %s", err)
		}
		if err := errors.New(reply.Error); err != nil {
			return proto.FastUpdateResponse{}, err
		}
		if reply.ProgressMessage != "" {
			logger.Debugln(0, reply.ProgressMessage)
		}
		if reply.Final {
			return reply, nil
		}
	}
}

func forceDisruptiveUpdate(client srpc.ClientI, subHostname string) error {
	request := proto.ForceDisruptiveUpdateRequest{Hostname: subHostname}
	var reply proto.ForceDisruptiveUpdateResponse
	return client.RequestReply("Dominator.ForceDisruptiveUpdate", request,
		&reply)
}

func getDefaultImage(client srpc.ClientI) (string, error) {
	var request proto.GetDefaultImageRequest
	var reply proto.GetDefaultImageResponse
	err := client.RequestReply("Dominator.GetDefaultImage", request, &reply)
	if err != nil {
		return "", err
	}
	return reply.ImageName, nil
}

func getInfoForSubs(client srpc.ClientI, request proto.GetInfoForSubsRequest) (
	proto.GetInfoForSubsResponse, error) {
	var reply proto.GetInfoForSubsResponse
	err := client.RequestReply("Dominator.GetInfoForSubs", request, &reply)
	if err != nil {
		return proto.GetInfoForSubsResponse{}, err
	}
	if err := errors.New(reply.Error); err != nil {
		return proto.GetInfoForSubsResponse{}, err
	}
	return reply, nil
}

func getSubsConfiguration(client srpc.ClientI) (subproto.Configuration, error) {
	var request proto.GetSubsConfigurationRequest
	var reply proto.GetSubsConfigurationResponse
	err := client.RequestReply("Dominator.GetSubsConfiguration", request,
		&reply)
	if err != nil {
		return subproto.Configuration{}, err
	}
	return subproto.Configuration(reply), nil
}

func listSubs(client srpc.ClientI, request proto.ListSubsRequest) (
	[]string, error) {
	var reply proto.ListSubsResponse
	err := client.RequestReply("Dominator.ListSubs", request, &reply)
	if err != nil {
		return nil, err
	}
	if err := errors.New(reply.Error); err != nil {
		return nil, err
	}
	return reply.Hostnames, nil
}

func setDefaultImage(client srpc.ClientI, imageName string) error {
	request := proto.SetDefaultImageRequest{ImageName: imageName}
	var reply proto.SetDefaultImageResponse
	err := client.RequestReply("Dominator.SetDefaultImage", request, &reply)
	return err
}
