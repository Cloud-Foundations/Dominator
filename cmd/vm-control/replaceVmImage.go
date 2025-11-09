package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func replaceVmImageSubcommand(args []string, logger log.DebugLogger) error {
	if err := replaceVmImage(args[0], logger); err != nil {
		return fmt.Errorf("error replacing VM image: %s", err)
	}
	return nil
}

func replaceVmImage(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return replaceVmImageOnHypervisor(hypervisor, vmIP, logger)
	}
}

func replaceVmImageOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	request := proto.ReplaceVmImageRequest{
		DhcpTimeout:      *dhcpTimeout,
		IpAddress:        ipAddr,
		MinimumFreeBytes: uint64(minFreeBytes),
		PreDelete:        *preDelete,
		RoundupPower:     *roundupPower,
		SkipBackup:       *skipBackup,
		VolumeFormat:     volumeFormat,
	}
	var imageReader io.Reader
	if *imageName != "" {
		request.ImageName = *imageName
		request.ImageTimeout = *imageTimeout
		request.SkipBootloader = *skipBootloader
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
			imageReader = bufio.NewReader(io.LimitReader(file, size))
		}
	} else {
		return errors.New("no image specified")
	}
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	dhcpTimedOut, err := hyperclient.ReplaceVmImage(client, request,
		imageReader, logger)
	if err != nil {
		return err
	}
	if dhcpTimedOut {
		return errors.New("DHCP ACK timed out")
	}
	return nil
}
