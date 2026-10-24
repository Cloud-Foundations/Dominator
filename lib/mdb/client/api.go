package client

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/mdbserver"
)

// GetMachine returns the response containing the MDB data for the machine with
// the specified hostname, or an error if the RPC fails or the server reports
// one.
func GetMachine(client srpc.ClientI,
	hostname string) (proto.GetMachineResponse, error) {
	return getMachine(client, hostname)
}

// GetMdb returns the response containing the MDB data for all machines, or an
// error if the RPC fails or the server reports one.
func GetMdb(client srpc.ClientI) (proto.GetMdbResponse, error) {
	return getMdb(client)
}

// GetMdbUpdates calls updateFunc with each MDB update streamed by the server.
// The first update contains the full MDB data. It returns when the stream
// fails or updateFunc returns an error, and always returns a non-nil error.
func GetMdbUpdates(client srpc.ClientI,
	updateFunc func(proto.MdbUpdate) error) error {
	return getMdbUpdates(client, updateFunc)
}

// GetFilteredMdbUpdates is like GetMdbUpdates but only streams machines in or
// below the specified locations. It always returns a non-nil error.
func GetFilteredMdbUpdates(client srpc.ClientI, locations []string,
	updateFunc func(proto.MdbUpdate) error) error {
	return getFilteredMdbUpdates(client, locations, updateFunc)
}

// ListImages returns the planned and required images in the MDB, or an error
// if the RPC fails.
func ListImages(client srpc.ClientI) (proto.ListImagesResponse, error) {
	return listImages(client)
}

// PauseUpdates pauses (or with request.Remove, unpauses) updates for a
// machine. It returns an error if the RPC fails or the server reports one.
func PauseUpdates(client srpc.ClientI,
	request proto.PauseUpdatesRequest) error {
	return pauseUpdates(client, request)
}

// ResumeUpdates resumes updates for the machine with the specified hostname.
// It returns an error if the RPC fails or the server reports one.
func ResumeUpdates(client srpc.ClientI, hostname string) error {
	return resumeUpdates(client, hostname)
}
