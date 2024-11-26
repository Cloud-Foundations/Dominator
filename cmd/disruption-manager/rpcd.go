package main

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	dm_proto "github.com/Cloud-Foundations/Dominator/proto/disruptionmanager"
	sub_proto "github.com/Cloud-Foundations/Dominator/proto/sub"
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
				"Cancel",
				"Check",
				"Request",
			}})
	return nil
}

func (t *rpcType) Cancel(conn *srpc.Conn,
	request dm_proto.DisruptionCancelRequest,
	reply *dm_proto.DisruptionCancelResponse) error {
	var err error
	var logMessage string
	var state sub_proto.DisruptionState
	authInfo := conn.GetAuthInformation()
	if authInfo == nil || !authInfo.HaveMethodAccess {
		err := hostAccessCheck(conn.RemoteAddr(), request.MDB.Hostname)
		if err != nil {
			reply.Error = err.Error()
			return nil
		}
	}
	state, logMessage, err = t.disruptionManager.cancel(request.MDB)
	reply.Error = errors.ErrorToString(err)
	reply.Response = state
	if logMessage != "" {
		t.disruptionManager.logger.Println(logMessage)
	}
	return nil
}

func (t *rpcType) Check(conn *srpc.Conn,
	request dm_proto.DisruptionCheckRequest,
	reply *dm_proto.DisruptionCheckResponse) error {
	state, logMessage, err := t.disruptionManager.check(request.MDB)
	reply.Error = errors.ErrorToString(err)
	reply.Response = state
	if logMessage != "" {
		t.disruptionManager.logger.Println(logMessage)
	}
	return nil
}

func (t *rpcType) Request(conn *srpc.Conn,
	request dm_proto.DisruptionRequestRequest,
	reply *dm_proto.DisruptionRequestResponse) error {
	var err error
	var logMessage string
	var state sub_proto.DisruptionState
	authInfo := conn.GetAuthInformation()
	if authInfo == nil || !authInfo.HaveMethodAccess {
		err := hostAccessCheck(conn.RemoteAddr(), request.MDB.Hostname)
		if err != nil {
			reply.Error = err.Error()
			return nil
		}
	}
	state, logMessage, err = t.disruptionManager.request(request.MDB)
	reply.Error = errors.ErrorToString(err)
	reply.Response = state
	if logMessage != "" {
		t.disruptionManager.logger.Println(logMessage)
	}
	return nil
}
