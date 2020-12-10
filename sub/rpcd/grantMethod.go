package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

func (t *rpcType) GrantMethod(serviceMethod string,
	authInfo *srpc.AuthInformation) bool {
	if authInfo == nil || authInfo.Username == "" {
		return false
	}
	if _, ok := t.ownerUsers[authInfo.Username]; ok {
		return true
	}
	for _, group := range t.subConfiguration.OwnerGroups {
		if _, ok := authInfo.GroupList[group]; ok {
			return true
		}
	}
	return false
}
