package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

var (
	_imageServerClient srpc.ClientI
)

func dialFleetManager(address string) (*srpc.Client, error) {
	return srpc.DialHTTPWithDialer("tcp", address, rrDialer)
}

func dialHypervisor(address string) (*srpc.Client, error) {
	return srpc.DialHTTP("tcp", address, 0)
}

// getImageServerClient will connect to the image server if specified and return
// the client. If no image server is specified, nil is returned. The client
// should not be closed.
func getImageServerClient() (srpc.ClientI, error) {
	if *imageServerHostname == "" {
		return nil, nil
	}
	if _imageServerClient != nil {
		return _imageServerClient, nil
	}
	address := fmt.Sprintf("%s:%d", *imageServerHostname, *imageServerPortNum)
	client, err := srpc.DialHTTPWithDialer("tcp", address, rrDialer)
	if err != nil {
		return nil, err
	}
	_imageServerClient = client
	return _imageServerClient, nil
}
