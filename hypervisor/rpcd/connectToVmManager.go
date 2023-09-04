package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) ConnectToVmManager(conn *srpc.Conn) error {
	var request hypervisor.ConnectToVmManagerRequest
	if err := conn.Decode(&request); err != nil {
		return err
	}
	input, output, err := t.manager.ConnectToVmManager(request.IpAddress)
	if input != nil {
		defer close(input)
	}
	e := conn.Encode(hypervisor.ConnectToVmManagerResponse{
		Error: errors.ErrorToString(err)})
	if e != nil {
		return e
	}
	if e := conn.Flush(); e != nil {
		return e
	}
	if err != nil {
		return nil // Have successfully sent the error in the response message.
	}
	return connectChannelsToConnection(conn, input, output)
}
