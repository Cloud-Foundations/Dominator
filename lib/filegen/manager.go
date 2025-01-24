package filegen

import (
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver/memory"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/filegenerator"
	"github.com/Cloud-Foundations/tricorder/go/tricorder"
)

type rpcType struct {
	manager *Manager
}

func newManager(logger log.Logger) *Manager {
	m := &Manager{
		bucketer: tricorder.NewGeometricBucketer(0.01, 1e5),
		clients: make(
			map[<-chan *proto.ServerMessage]chan<- *proto.ServerMessage),
		logger:       logger,
		machineData:  make(map[string]mdb.Machine),
		objectServer: memory.NewObjectServer(),
		pathManagers: make(map[string]*pathManager),
	}
	m.registerMdbGeneratorForPath("/etc/mdb.json")
	srpc.RegisterNameWithOptions("FileGenerator", &rpcType{m},
		srpc.ReceiverOptions{
			PublicMethods: []string{
				"ListGenerators",
			}})
	return m
}

func (t *rpcType) ListGenerators(conn *srpc.Conn,
	request proto.ListGeneratorsRequest,
	reply *proto.ListGeneratorsResponse) error {
	reply.Pathnames = t.manager.GetRegisteredPaths()
	return nil
}
