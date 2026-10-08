package mdbserver

import (
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/mdb"
)

type GetMachineRequest struct {
	Hostname string
}

type GetMachineResponse struct {
	Error   string
	Machine mdb.Machine
}

type GetMdbRequest struct{}

type GetMdbResponse struct {
	Error    string
	Machines []mdb.Machine
}

// The GetMdbUpdates() RPC is fully streamed.
// The client sends no information to the server.
// The server sends a stream of MdbUpdate messages.
// At connection start, the full MDB data are presented in .MachinesToAdd and
// .MachinesToUpdate and .MachinesToDelete will be nil.

type MdbUpdate struct {
	MachinesToAdd    []mdb.Machine
	MachinesToUpdate []mdb.Machine
	MachinesToDelete []string
}

// The GetFilteredMdbUpdates() RPC is fully streamed.
// The client sends a single GetFilteredMdbUpdatesRequest message.
// The server sends a stream of MdbUpdate messages for machines in or below the
// requested locations. If no locations are requested, all machines are sent.
// At connection start, the matching MDB data are presented in .MachinesToAdd
// and .MachinesToUpdate and .MachinesToDelete will be nil.

type GetFilteredMdbUpdatesRequest struct {
	Locations []string
}

type ListImagesRequest struct{}

type ListImagesResponse struct {
	PlannedImages  []string
	RequiredImages []string
}

type PauseUpdatesRequest struct {
	Hostname string
	Reason   string
	Remove   bool
	Until    time.Time
}

type PauseUpdatesResponse struct {
	Error string
}

type ResumeUpdatesRequest struct {
	Hostname string
}

type ResumeUpdatesResponse struct {
	Error string
}
