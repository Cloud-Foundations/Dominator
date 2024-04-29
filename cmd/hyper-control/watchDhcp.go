package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
	dhcp "github.com/krolaw/dhcp4"
)

func watchDhcpSubcommand(args []string, logger log.DebugLogger) error {
	var interfaceName string
	if len(args) > 0 {
		interfaceName = args[0]
	}
	if err := watchDhcp(interfaceName, logger); err != nil {
		return fmt.Errorf("error watching DHCP: %s", err)
	}
	return nil
}

func watchDhcp(interfaceName string, logger log.DebugLogger) error {
	if *hypervisorHostname == "" {
		return errors.New("unspecified Hypervisor")
	}
	clientName := fmt.Sprintf("%s:%d", *hypervisorHostname, *hypervisorPortNum)
	client, err := srpc.DialHTTP("tcp", clientName, 0)
	if err != nil {
		return err
	}
	defer client.Close()
	conn, err := client.Call("Hypervisor.WatchDhcp")
	if err != nil {
		return err
	}
	defer conn.Close()
	request := proto.WatchDhcpRequest{Interface: interfaceName}
	if err := conn.Encode(request); err != nil {
		return err
	}
	if err := conn.Flush(); err != nil {
		return err
	}
	dirname, err := os.MkdirTemp("",
		"hyper-control.watch-dhcp."+*hypervisorHostname+".")
	if err != nil {
		return err
	}
	logger.Printf("Results in directory: %s\n", dirname)
	for counter := 0; true; counter++ {
		var reply proto.WatchDhcpResponse
		if err := conn.Decode(&reply); err != nil {
			return err
		}
		if err := errors.New(reply.Error); err != nil {
			return err
		}
		filename := fmt.Sprintf("%s.%.5d", reply.Interface, counter)
		file, err := os.Create(filepath.Join(dirname, filename))
		if err != nil {
			return err
		}
		file.Write(reply.Packet)
		file.Close()
		packet := dhcp.Packet(reply.Packet)
		options := packet.ParseOptions()
		msgType := dhcp.MessageType(options[dhcp.OptionDHCPMessageType][0])
		logger.Printf("Counter: %d, message type: %s, from: %s, options:\n",
			counter, msgType, packet.CHAddr())
		optionsDirname := filepath.Join(dirname, filename) + ".options"
		os.Mkdir(optionsDirname, fsutil.DirPerms)
		for code, value := range options {
			logger.Printf("  Code: %s, value: %0x\n", code, value)
			optionFilename := fmt.Sprintf("%s/%d_%s",
				optionsDirname, code, code)
			file, err := os.Create(optionFilename)
			if err != nil {
				return err
			}
			file.Write(value)
			file.Close()
		}
	}
	return nil
}
