package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) PurgeVolumeCache(conn *srpc.Conn,
	request hypervisor.PurgeVolumeCacheRequest,
	reply *hypervisor.PurgeVolumeCacheResponse) error {
	authInfo := conn.GetAuthInformation()
	if resp, err := t.manager.PurgeVolumeCache(request, authInfo); err != nil {
		reply.Error = errors.ErrorToString(err)
	} else {
		*reply = resp
	}
	return nil
}
