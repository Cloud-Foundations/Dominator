package disruptionmanager

import (
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
	"github.com/Cloud-Foundations/Dominator/proto/sub"
)

type DisruptionRequest struct {
	MDB     mdb.Machine
	Request sub.DisruptionRequest
}

type DisruptionResponse struct {
	Response sub.DisruptionState
}

type RequestType uint
