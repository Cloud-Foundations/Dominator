package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) FetchImage(conn *srpc.Conn,
	request hypervisor.FetchImageRequest,
	reply *hypervisor.FetchImageResponse) error {
	resp, err := t.manager.FetchImage(request.SearchName, request.ImageTimeout)
	if err != nil {
		reply.Error = errors.ErrorToString(err)
	} else {
		*reply = resp
	}
	return nil
}
