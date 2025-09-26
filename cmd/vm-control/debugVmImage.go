package main

import (
	"fmt"
	"io"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func debugVmImageSubcommand(args []string, logger log.DebugLogger) error {
	if err := debugVmImage(args[0], logger); err != nil {
		return fmt.Errorf("error starting VM with debug image: %s", err)
	}
	return nil
}

func debugVmImage(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return debugVmImageOnHypervisor(hypervisor, vmIP, logger)
	}
}

func debugVmImageOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	request := proto.DebugVmImageRequest{
		DhcpTimeout:      *dhcpTimeout,
		IpAddress:        ipAddr,
		MinimumFreeBytes: uint64(minFreeBytes),
		RoundupPower:     *roundupPower,
	}
	var imageReader io.Reader
	if *imageName != "" {
		request.ImageName = *imageName
		request.ImageTimeout = *imageTimeout
		if overlayFiles, err := loadOverlayFiles(); err != nil {
			return err
		} else {
			request.OverlayFiles = overlayFiles
		}
	} else if *imageURL != "" {
		request.ImageURL = *imageURL
	} else if *imageFile != "" {
		file, size, err := getReader(*imageFile)
		if err != nil {
			return err
		} else {
			defer file.Close()
			request.ImageDataSize = uint64(size)
			imageReader = file
		}
	} else {
		return errors.New("no image specified")
	}
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	dhcpTimedOut, err := hyperclient.DebugVmImage(client, request, imageReader,
		int64(request.ImageDataSize), logger)
	if err != nil {
		return err
	}
	if dhcpTimedOut {
		return errors.New("DHCP ACK timed out")
	}
	if *dhcpTimeout > 0 {
		logger.Debugln(0, "Received DHCP ACK")
	}
	return maybeWatchVm(client, hypervisor, ipAddr, logger)
}
