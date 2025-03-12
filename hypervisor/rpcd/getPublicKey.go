package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) GetPublicKey(conn *srpc.Conn,
	request hypervisor.GetPublicKeyRequest,
	reply *hypervisor.GetPublicKeyResponse) error {
	pubkey, err := t.manager.GetPublicKey()
	if err != nil {
		reply.Error = err.Error()
	} else {
		reply.KeyPEM = pubkey
	}
	return nil
}
