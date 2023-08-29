package main

import (
	"fmt"
	"io"
	"net"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
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
	var reply proto.DebugVmImageResponse
	err = callDebugVmImage(client, request, &reply, imageReader,
		int64(request.ImageDataSize), logger)
	if err != nil {
		return err
	}
	if reply.DhcpTimedOut {
		return errors.New("DHCP ACK timed out")
	}
	if *dhcpTimeout > 0 {
		logger.Debugln(0, "Received DHCP ACK")
	}
	return maybeWatchVm(client, hypervisor, ipAddr, logger)
}

func callDebugVmImage(client *srpc.Client,
	request proto.DebugVmImageRequest,
	reply *proto.DebugVmImageResponse, imageReader io.Reader,
	imageSize int64, logger log.DebugLogger) error {
	conn, err := client.Call("Hypervisor.DebugVmImage")
	if err != nil {
		return fmt.Errorf("error calling Hypervisor.DebugVmImage: %s", err)
	}
	defer conn.Close()
	if err := conn.Encode(request); err != nil {
		return fmt.Errorf("error encoding request: %s", err)
	}
	// Stream any required data.
	if imageReader != nil {
		logger.Debugln(0, "uploading image")
		startTime := time.Now()
		if nCopied, err := io.CopyN(conn, imageReader, imageSize); err != nil {
			return fmt.Errorf("error uploading image: %s got %d of %d bytes",
				err, nCopied, imageSize)
		} else {
			duration := time.Since(startTime)
			speed := uint64(float64(nCopied) / duration.Seconds())
			logger.Debugf(0, "uploaded image in %s (%s/s)\n",
				format.Duration(duration), format.FormatBytes(speed))
		}
	}
	response, err := processDebugVmImageResponses(conn, logger)
	*reply = response
	return err
}

func processDebugVmImageResponses(conn *srpc.Conn,
	logger log.DebugLogger) (proto.DebugVmImageResponse, error) {
	var zeroResponse proto.DebugVmImageResponse
	if err := conn.Flush(); err != nil {
		return zeroResponse, fmt.Errorf("error flushing: %s", err)
	}
	for {
		var response proto.DebugVmImageResponse
		if err := conn.Decode(&response); err != nil {
			return zeroResponse, fmt.Errorf("error decoding: %s", err)
		}
		if response.Error != "" {
			return zeroResponse, errors.New(response.Error)
		}
		if response.ProgressMessage != "" {
			logger.Debugln(0, response.ProgressMessage)
		}
		if response.Final {
			return response, nil
		}
	}
}
