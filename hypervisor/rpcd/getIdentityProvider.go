package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) GetIdentityProvider(conn *srpc.Conn,
	request hypervisor.GetIdentityProviderRequest,
	reply *hypervisor.GetIdentityProviderResponse) error {
	identityProvider, err := t.manager.GetIdentityProvider()
	if err != nil {
		reply.Error = err.Error()
	} else {
		reply.BaseUrl = identityProvider
	}
	return nil
}
