package rpcd

import (
	"io"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/tricorder/go/tricorder"
	"github.com/Cloud-Foundations/tricorder/go/tricorder/units"
)

type Config struct {
	AllowPublicAddObjects   bool
	AllowPublicCheckObjects bool
	AllowPublicGetObjects   bool
	ReplicationMaster       string
}

type Params struct {
	Logger       log.DebugLogger
	ObjectServer objectserver.StashingObjectServer
}

type srpcType struct {
	objectServer      objectserver.StashingObjectServer
	replicationMaster string
	getSemaphore      chan bool
	logger            log.DebugLogger
}

type htmlWriter struct {
	getSemaphore chan bool
}

func (hw *htmlWriter) WriteHtml(writer io.Writer) {
	hw.writeHtml(writer)
}

func Setup(config Config, params Params) *htmlWriter {
	getSemaphore := make(chan bool, 100)
	srpcObj := &srpcType{
		objectServer:      params.ObjectServer,
		replicationMaster: config.ReplicationMaster,
		getSemaphore:      getSemaphore,
		logger:            params.Logger,
	}
	var publicMethods []string
	if config.AllowPublicAddObjects {
		publicMethods = append(publicMethods, "AddObjects")
	}
	if config.AllowPublicCheckObjects {
		publicMethods = append(publicMethods, "CheckObjects")
	}
	if config.AllowPublicGetObjects {
		publicMethods = append(publicMethods, "GetObjects")
	}
	srpc.RegisterNameWithOptions("ObjectServer", srpcObj,
		srpc.ReceiverOptions{PublicMethods: publicMethods})
	tricorder.RegisterMetric("/get-requests",
		func() uint { return uint(len(getSemaphore)) },
		units.None, "number of GetObjects() requests in progress")
	return &htmlWriter{getSemaphore}
}
