package mdbserver

import (
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/mdb"
)

type ListImagesRequest struct{}

type ListImagesResponse struct {
	PlannedImages  []string
	RequiredImages []string
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

type PauseUpdatesRequest struct {
	Hostname string
	Reason   string
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
