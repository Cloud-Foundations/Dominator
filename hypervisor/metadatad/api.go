package metadatad

import (
	"io"
	"net"
	"net/http"

	"github.com/Cloud-Foundations/Dominator/hypervisor/manager"
	"github.com/Cloud-Foundations/Dominator/lib/constants"
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
	fileHandlers      map[string]string
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
	s.fileHandlers = map[string]string{
		constants.MetadataIdentityCert: manager.IdentityCertFile,
		constants.MetadataIdentityKey:  manager.IdentityKeyFile,
		constants.MetadataUserData:     manager.UserDataFile,
	}
	s.infoHandlers = map[string]metadataWriter{
		constants.MetadataEpochTime:   s.showTime,
		constants.MetadataIdentityDoc: s.showVM,
	}
	s.rawHandlers = map[string]rawHandlerFunc{
		constants.SmallStackDataSource:        s.showTrue,
		constants.MetadataExternallyPatchable: s.showTrue,
	}
	s.computePaths()
	return s.startServer()
}
