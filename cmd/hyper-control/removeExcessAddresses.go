package main

import (
	"fmt"
	"strconv"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func removeExcessAddressesSubcommand(args []string,
	logger log.DebugLogger) error {
	err := removeExcessAddresses(args[0], logger)
	if err != nil {
		return fmt.Errorf("error removing excess addresses: %s", err)
	}
	return nil
}

func removeExcessAddresses(maxAddr string, logger log.DebugLogger) error {
	maxAddresses, err := strconv.ParseUint(maxAddr, 10, 16)
	if err != nil {
		return err
	}
	request := proto.ChangeAddressPoolRequest{
		MaximumFreeAddresses: map[string]uint{"": uint(maxAddresses)}}
	var reply proto.ChangeAddressPoolResponse
	clientName := fmt.Sprintf("%s:%d", *hypervisorHostname, *hypervisorPortNum)
	client, err := srpc.DialHTTP("tcp", clientName, 0)
	if err != nil {
		return err
	}
	defer client.Close()
	err = client.RequestReply("Hypervisor.ChangeAddressPool",
		request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}
