package main

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/disruptionmanager"
)

type rpcType struct {
	disruptionManager *disruptionManager
	logger            log.DebugLogger
}

func startRpcServer(dm *disruptionManager, logger log.DebugLogger) error {
	rpcObj := &rpcType{
		disruptionManager: dm,
		logger:            logger,
	}
	srpc.RegisterNameWithOptions("DisruptionManager", rpcObj,
		srpc.ReceiverOptions{
			PublicMethods: []string{
				"Check",
			}})
	return nil
}

func (t *rpcType) Cancel(conn *srpc.Conn, request proto.DisruptionCancelRequest,
	reply *proto.DisruptionCancelResponse) error {
	state, logMessage, err := t.disruptionManager.cancel(request.MDB)
	reply.Error = errors.ErrorToString(err)
	reply.Response = state
	if logMessage != "" {
		t.disruptionManager.logger.Println(logMessage)
	}
	return nil
}

func (t *rpcType) Check(conn *srpc.Conn, request proto.DisruptionCheckRequest,
	reply *proto.DisruptionCheckResponse) error {
	state, logMessage, err := t.disruptionManager.check(request.MDB)
	reply.Error = errors.ErrorToString(err)
	reply.Response = state
	if logMessage != "" {
		t.disruptionManager.logger.Println(logMessage)
	}
	return nil
}

func (t *rpcType) Request(conn *srpc.Conn,
	request proto.DisruptionRequestRequest,
	reply *proto.DisruptionRequestResponse) error {
	state, logMessage, err := t.disruptionManager.request(request.MDB)
	reply.Error = errors.ErrorToString(err)
	reply.Response = state
	if logMessage != "" {
		t.disruptionManager.logger.Println(logMessage)
	}
	return nil
}
