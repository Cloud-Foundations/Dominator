package manager

import (
	"path/filepath"

	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (m *Manager) replaceVmCredentials(
	request proto.ReplaceVmCredentialsRequest,
	authInfo *srpc.AuthInformation) error {
	vm, err := m.getVmLockAndAuth(request.IpAddress, true, authInfo, nil)
	if err != nil {
		return err
	}
	defer vm.mutex.Unlock()
	identityName, identityExpires, err := validateIdentityKeyPair(
		request.IdentityCertificate, request.IdentityKey, authInfo.Username)
	if err != nil {
		return err
	}
	err = writeKeyPair(request.IdentityCertificate, request.IdentityKey,
		filepath.Join(vm.dirname, IdentityCertFile),
		filepath.Join(vm.dirname, IdentityKeyFile))
	if err != nil {
		return err
	}
	if !vm.IdentityExpires.Equal(identityExpires) ||
		vm.IdentityName != identityName {
		vm.IdentityExpires = identityExpires
		vm.IdentityName = identityName
		vm.writeAndSendInfo()
	}
	return nil
}
