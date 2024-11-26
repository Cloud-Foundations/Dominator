package main

import (
	"errors"
	"fmt"

	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

func getMdbdClient() (srpc.ClientI, error) {
	hostname := *mdbServerHostname
	if hostname == "" {
		hostname = *domHostname
	}
	if hostname == "" {
		return nil, errors.New("no MDB server hostname specified")
	}
	clientName := fmt.Sprintf("%s:%d", hostname, *mdbServerPortNum)
	client, err := srpc.DialHTTP("tcp", clientName, 0)
	if err != nil {
		return nil, fmt.Errorf("error dialing: %s: %s", clientName, err)
	}
	return client, nil
}
