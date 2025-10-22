package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/sub"
	subclient "github.com/Cloud-Foundations/Dominator/sub/client"
)

func getImageServerAddress() string {
	hostname := *imageServerHostname
	if hostname == "" {
		hostname = "localhost"
	}
	return fmt.Sprintf("%s:%d", hostname, *imageServerPortNum)
}

func getObjectServerAddress() string {
	if *objectServerHostname == "" {
		return getImageServerAddress()
	}
	return fmt.Sprintf("%s:%d",
		*objectServerHostname, *objectServerPortNum)
}

func getSubImage(srpcClient *srpc.Client) (string, error) {
	var response proto.PollResponse
	err := subclient.CallPoll(srpcClient,
		proto.PollRequest{ShortPollOnly: true},
		&response)
	if err != nil {
		return "", err
	}
	if response.LastSuccessfulImageName != "" {
		return response.LastSuccessfulImageName, nil
	}
	return response.InitialImageName, nil
}
