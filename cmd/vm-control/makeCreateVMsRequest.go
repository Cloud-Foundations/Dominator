package main

import (
	"fmt"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func makeCreateVMsRequestSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := showCreateVMsRequest(logger); err != nil {
		return fmt.Errorf("error making VMs create request: %s", err)
	}
	return nil
}

func showCreateVMsRequest(logger log.DebugLogger) error {
	requests, err := makeCreateVMsRequest(logger)
	if err != nil {
		return err
	}
	return json.WriteWithIndent(os.Stdout, "    ", requests)
}
