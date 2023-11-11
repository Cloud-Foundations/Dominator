package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) DestroyVm(conn *srpc.Conn,
	request hypervisor.DestroyVmRequest,
	reply *hypervisor.DestroyVmResponse) error {
	authInfo := conn.GetAuthInformation()
	t.logger.Debugf(1, "DestroyVm(%s) starting, IP=%s\n",
		authInfo.Username, request.IpAddress)
	err := t.manager.DestroyVm(request.IpAddress, authInfo, request.AccessToken)
	if err == nil {
		t.logger.Debugf(1, "DestroyVm(%s) finished, IP=%s\n",
			authInfo.Username, request.IpAddress)
	} else {
		t.logger.Debugf(1, "DestroyVm(%s) failed, IP=%s, error: %s\n",
			authInfo.Username, request.IpAddress, err)
	}
	response := hypervisor.DestroyVmResponse{errors.ErrorToString(err)}
	*reply = response
	return nil
}
