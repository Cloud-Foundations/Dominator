package client

import (
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/dominator"
	subproto "github.com/Cloud-Foundations/Dominator/proto/sub"
)

func ClearSafetyShutoff(client srpc.ClientI, subHostname string) error {
	return clearSafetyShutoff(client, subHostname)
}

func ConfigureSubs(client srpc.ClientI,
	configuration subproto.Configuration) error {
	return configureSubs(client, configuration)
}

func DisableUpdates(client srpc.ClientI, reason string) error {
	return disableUpdates(client, reason)
}

func EnableUpdates(client srpc.ClientI, reason string) error {
	return enableUpdates(client, reason)
}

func FastUpdate(client srpc.ClientI, request proto.FastUpdateRequest,
	logger log.DebugLogger) (bool, error) {
	reply, err := fastUpdate(client, request, logger)
	return reply.Synced, err
}

func FastUpdateDetailed(client srpc.ClientI, request proto.FastUpdateRequest,
	logger log.DebugLogger) (proto.FastUpdateResponse, error) {
	return fastUpdate(client, request, logger)
}

func ForceDisruptiveUpdate(client srpc.ClientI, subHostname string) error {
	return forceDisruptiveUpdate(client, subHostname)
}

func GetDefaultImage(client srpc.ClientI) (string, error) {
	return getDefaultImage(client)
}

func GetInfoForSubs(client srpc.ClientI, request proto.GetInfoForSubsRequest) (
	proto.GetInfoForSubsResponse, error) {
	return getInfoForSubs(client, request)
}

func GetSubsConfiguration(client srpc.ClientI) (subproto.Configuration, error) {
	return getSubsConfiguration(client)
}

func ListSubs(client srpc.ClientI, request proto.ListSubsRequest) (
	[]string, error) {
	return listSubs(client, request)
}

func SetDefaultImage(client srpc.ClientI, imageName string) error {
	return setDefaultImage(client, imageName)
}
