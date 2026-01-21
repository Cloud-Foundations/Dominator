package main

import (
	"fmt"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	terminalclient "github.com/Cloud-Foundations/Dominator/lib/net/terminal/client"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

func shellSubcommand(args []string, logger log.DebugLogger) error {
	srpcClient := getSubClient(logger)
	defer srpcClient.Close()
	err := shell(srpcClient, logger)
	if err != nil {
		return fmt.Errorf("error talking to subd shell: %s", err)
	}
	return nil
}

func shell(client srpc.ClientI, logger log.DebugLogger) error {
	conn, err := client.Call("Subd.Shell")
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := terminalclient.StartTerminal(conn); err != nil {
		return err
	}
	fmt.Fprint(os.Stderr, "\r")
	return nil
}
