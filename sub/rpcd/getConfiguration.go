package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/sub"
)

func (t *rpcType) GetConfiguration(conn *srpc.Conn,
	request sub.GetConfigurationRequest,
	reply *sub.GetConfigurationResponse) error {
	var response sub.GetConfigurationResponse
	response = sub.GetConfigurationResponse(t.getConfiguration())
	*reply = response
	return nil
}

func (t *rpcType) getConfiguration() sub.Configuration {
	var configuration sub.Configuration
	configuration.CpuPercent =
		t.params.ScannerConfiguration.DefaultCpuPercent
	configuration.NetworkSpeedPercent =
		t.params.ScannerConfiguration.NetworkReaderContext.SpeedPercent()
	configuration.ScanSpeedPercent =
		t.params.ScannerConfiguration.FsScanContext.GetContext().SpeedPercent()
	configuration.ScanExclusionList =
		t.params.ScannerConfiguration.ScanFilter.FilterLines
	return configuration
}
