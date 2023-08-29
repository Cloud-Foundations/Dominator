package sub

import (
	"fmt"
)

func (configuration Configuration) String() string {
	retval := fmt.Sprintf("CpuPercent: %d\nNetworkSpeedPercent: %d\nScanSpeedPercent: %d",
		configuration.CpuPercent, configuration.NetworkSpeedPercent,
		configuration.ScanSpeedPercent)
	if len(configuration.ScanExclusionList) > 0 {
		retval += "\n" + "ScanExclusionList:"
		for _, exclusion := range configuration.ScanExclusionList {
			retval += "\n  " + exclusion
		}
	}
	if len(configuration.OwnerGroups) > 0 {
		retval += fmt.Sprintf("\nOwnerGroups: %v", configuration.OwnerGroups)
	}
	if len(configuration.OwnerUsers) > 0 {
		retval += fmt.Sprintf("\nOwnerUsers: %v", configuration.OwnerUsers)
	}
	return retval
}

func (configuration GetConfigurationResponse) String() string {
	return Configuration(configuration).String()
}
