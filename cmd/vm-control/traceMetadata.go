package main

import (
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

func traceVmMetadataSubcommand(args []string, logger log.DebugLogger) error {
	if err := traceVmMetadata(args[0], logger); err != nil {
		return fmt.Errorf("error tracing VM metadata: %s", err)
	}
	return nil
}

func traceVmMetadata(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return traceVmMetadataOnHypervisor(hypervisor, vmIP, logger)
	}
}

func traceVmMetadataOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	return doTraceMetadata(client, ipAddr, logger)
}

func doTraceMetadata(client *srpc.Client, ipAddr net.IP,
	logger log.Logger) error {
	return hyperclient.TraceVmMetadata(client, ipAddr, func(path string) error {
		logger.Println(path)
		return nil
	})
}

func maybeWatchVm(client *srpc.Client, hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	if !*traceMetadata && *probePortNum < 1 {
		return nil
	} else if *traceMetadata && *probePortNum < 1 {
		return doTraceMetadata(client, ipAddr, logger)
	} else if !*traceMetadata && *probePortNum > 0 {
		return probeVmPortOnHypervisorClient(client, ipAddr, logger)
	} else { // Do both.
		go doTraceMetadata(client, ipAddr, logger)
		return probeVmPortOnHypervisor(hypervisor, ipAddr, logger)
	}
}
