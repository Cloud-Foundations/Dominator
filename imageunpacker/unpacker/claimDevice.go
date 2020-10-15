package unpacker

import (
	"strings"
)

func (u *Unpacker) claimDevice(deviceId, deviceName string) error {
	if strings.HasPrefix(deviceName, "/dev/") {
		deviceName = deviceName[5:]
	}
	u.rwMutex.Lock()
	defer u.rwMutex.Unlock()
	defer u.updateUsageTimeWithLock()
	return u.addSpecfiedDevice(deviceId, deviceName)
}
