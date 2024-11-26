package disruptionmanager

import (
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
	"github.com/Cloud-Foundations/Dominator/proto/sub"
)

// DisruptionCancel RPC request.
type DisruptionCancelRequest struct {
	MDB mdb.Machine
}

// DisruptionCancel RPC response.
type DisruptionCancelResponse struct {
	Error    string
	Response sub.DisruptionState
}

// DisruptionCheck RPC request.
type DisruptionCheckRequest struct {
	MDB mdb.Machine
}

// DisruptionCheck RPC response.
type DisruptionCheckResponse struct {
	Error    string
	Response sub.DisruptionState
}

// REST endpoint request.
type DisruptionRequest struct {
	MDB     mdb.Machine
	Request sub.DisruptionRequest
}

// REST endpoint response.
type DisruptionResponse struct {
	Response sub.DisruptionState
}

// DisruptionCheck RPC request.
type DisruptionRequestRequest struct {
	MDB mdb.Machine
}

// DisruptionRequest RPC response.
type DisruptionRequestResponse struct {
	Error    string
	Response sub.DisruptionState
}

type RequestType uint
