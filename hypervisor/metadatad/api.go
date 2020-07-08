package metadatad

import (
	"io"
	"net"
	"net/http"

	"github.com/Cloud-Foundations/Dominator/hypervisor/manager"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

type rawHandlerFunc func(w http.ResponseWriter, ipAddr net.IP)
type metadataWriter func(writer io.Writer, vmInfo proto.VmInfo) error

type server struct {
	bridges           []net.Interface
	hypervisorPortNum uint
	manager           *manager.Manager
	logger            log.DebugLogger
	infoHandlers      map[string]metadataWriter
	rawHandlers       map[string]rawHandlerFunc
	paths             map[string]struct{}
}

func StartServer(hypervisorPortNum uint, bridges []net.Interface,
	managerObj *manager.Manager, logger log.DebugLogger) error {
	s := &server{
		bridges:           bridges,
		hypervisorPortNum: hypervisorPortNum,
		manager:           managerObj,
		logger:            logger,
	}
	s.infoHandlers = map[string]metadataWriter{
		"/latest/dynamic/epoch-time":                 s.showTime,
		"/latest/dynamic/instance-identity/document": s.showVM,
	}
	s.rawHandlers = map[string]rawHandlerFunc{
		"/datasource/SmallStack":          s.showTrue,
		"/latest/is-externally-patchable": s.showTrue,
		"/latest/user-data":               s.showUserData,
	}
	s.computePaths()
	return s.startServer()
}
