package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) CacheVolume(conn *srpc.Conn,
	request hypervisor.CacheVolumeRequest,
	reply *hypervisor.CacheVolumeResponse) error {
	if resp, err := t.manager.CacheVolume(request); err != nil {
		reply.Error = errors.ErrorToString(err)
	} else {
		*reply = resp
	}
	return nil
}
