package client

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/mdbserver"
)

// getMachine returns the machine data, or the RPC or server error.
func getMachine(client srpc.ClientI, hostname string) (mdb.Machine, error) {
	request := proto.GetMachineRequest{Hostname: hostname}
	var reply proto.GetMachineResponse
	err := client.RequestReply("MdbServer.GetMachine", request, &reply)
	if err != nil {
		return mdb.Machine{}, err
	}
	if err := errors.New(reply.Error); err != nil {
		return mdb.Machine{}, err
	}
	return reply.Machine, nil
}

// getMdb returns all machines, or the RPC or server error.
func getMdb(client srpc.ClientI) ([]mdb.Machine, error) {
	request := proto.GetMdbRequest{}
	var reply proto.GetMdbResponse
	err := client.RequestReply("MdbServer.GetMdb", request, &reply)
	if err != nil {
		return nil, err
	}
	if err := errors.New(reply.Error); err != nil {
		return nil, err
	}
	return reply.Machines, nil
}

// getMdbUpdates returns the error that ended the update stream.
func getMdbUpdates(client srpc.ClientI,
	updateFunc func(proto.MdbUpdate) error) error {
	conn, err := client.Call("MdbServer.GetMdbUpdates")
	if err != nil {
		return err
	}
	defer conn.Close()
	return receiveMdbUpdates(conn, updateFunc)
}

// getFilteredMdbUpdates returns the error that ended the update stream.
func getFilteredMdbUpdates(client srpc.ClientI, locations []string,
	updateFunc func(proto.MdbUpdate) error) error {
	conn, err := client.Call("MdbServer.GetFilteredMdbUpdates")
	if err != nil {
		return err
	}
	defer conn.Close()
	request := proto.GetFilteredMdbUpdatesRequest{Locations: locations}
	if err := conn.Encode(request); err != nil {
		return err
	}
	if err := conn.Flush(); err != nil {
		return err
	}
	return receiveMdbUpdates(conn, updateFunc)
}

// listImages returns the planned and required images, or the RPC error.
func listImages(client srpc.ClientI) (proto.ListImagesResponse, error) {
	request := proto.ListImagesRequest{}
	var reply proto.ListImagesResponse
	err := client.RequestReply("MdbServer.ListImages", request, &reply)
	if err != nil {
		return proto.ListImagesResponse{}, err
	}
	return reply, nil
}

// pauseUpdates returns the RPC or server error, or nil.
func pauseUpdates(client srpc.ClientI,
	request proto.PauseUpdatesRequest) error {
	var reply proto.PauseUpdatesResponse
	err := client.RequestReply("MdbServer.PauseUpdates", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

// receiveMdbUpdates decodes updates from conn and passes each to updateFunc.
// It returns the decode error or the error from updateFunc.
func receiveMdbUpdates(conn *srpc.Conn,
	updateFunc func(proto.MdbUpdate) error) error {
	for {
		var mdbUpdate proto.MdbUpdate
		if err := conn.Decode(&mdbUpdate); err != nil {
			return err
		}
		if err := updateFunc(mdbUpdate); err != nil {
			return err
		}
	}
}

// resumeUpdates returns the RPC or server error, or nil.
func resumeUpdates(client srpc.ClientI, hostname string) error {
	request := proto.ResumeUpdatesRequest{Hostname: hostname}
	var reply proto.ResumeUpdatesResponse
	err := client.RequestReply("MdbServer.ResumeUpdates", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}
