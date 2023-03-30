package manager

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
)

func (m *Manager) changeOwners(ownerGroups, ownerUsers []string) error {
	ownerGroupsMap := stringutil.ConvertListToMap(ownerGroups, false)
	ownerUsersMap := stringutil.ConvertListToMap(ownerUsers, false)
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.ownerGroups = ownerGroupsMap
	m.ownerUsers = ownerUsersMap
	return nil
}

func (m *Manager) checkOwnership(authInfo *srpc.AuthInformation) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if authInfo.Username != "" {
		if _, ok := m.ownerUsers[authInfo.Username]; ok {
			return true
		}
	}
	for group := range authInfo.GroupList {
		if _, ok := m.ownerGroups[group]; ok {
			return true
		}
	}
	return false
}
