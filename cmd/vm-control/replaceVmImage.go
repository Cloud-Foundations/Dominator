package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func replaceVmImageSubcommand(args []string, logger log.DebugLogger) error {
	if err := replaceVmImage(args[0], logger); err != nil {
		return fmt.Errorf("error replacing VM image: %s", err)
	}
	return nil
}

func callReplaceVmImage(client *srpc.Client,
	request proto.ReplaceVmImageRequest, reply *proto.ReplaceVmImageResponse,
	imageReader io.Reader, logger log.DebugLogger) error {
	conn, err := client.Call("Hypervisor.ReplaceVmImage")
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.Encode(request); err != nil {
		return err
	}
	// Stream any required data.
	if imageReader != nil {
		logger.Debugln(0, "uploading image")
		if _, err := io.Copy(conn, imageReader); err != nil {
			return err
		}
	}
	if err := conn.Flush(); err != nil {
		return err
	}
	for {
		var response proto.ReplaceVmImageResponse
		if err := conn.Decode(&response); err != nil {
			return err
		}
		if response.Error != "" {
			return errors.New(response.Error)
		}
		if response.ProgressMessage != "" {
			logger.Debugln(0, response.ProgressMessage)
		}
		if response.Final {
			*reply = response
			return nil
		}
	}
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
		RoundupPower:     *roundupPower,
		SkipBackup:       *skipBackup,
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
	var reply proto.ReplaceVmImageResponse
	err = callReplaceVmImage(client, request, &reply, imageReader, logger)
	if err != nil {
		return err
	}
	return nil
}
