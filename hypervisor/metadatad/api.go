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
		constants.MetadataIdentityEd25519SshCert:  manager.IdentityEd25519SshCertFile,
		constants.MetadataIdentityEd25519SshKey:   manager.IdentityEd25519SshKeyFile,
		constants.MetadataIdentityEd25519X509Cert: manager.IdentityEd25519X509CertFile,
		constants.MetadataIdentityEd25519X509Key:  manager.IdentityEd25519X509KeyFile,
		constants.MetadataIdentityCert:            manager.IdentityRsaX509CertFile,
		constants.MetadataIdentityKey:             manager.IdentityRsaX509KeyFile,
		constants.MetadataIdentityRsaSshCert:      manager.IdentityRsaSshCertFile,
		constants.MetadataIdentityRsaSshKey:       manager.IdentityRsaSshKeyFile,

		constants.MetadataIdentityRsaX509Cert: manager.IdentityRsaX509CertFile,
		constants.MetadataIdentityRsaX509Key:  manager.IdentityRsaX509KeyFile,
		constants.MetadataUserData:            manager.UserDataFile,
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
