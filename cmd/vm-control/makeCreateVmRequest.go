package main

import (
	"fmt"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func makeCreateVmRequestSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := showCreateVmRequest(logger); err != nil {
		return fmt.Errorf("error making VM create request: %s", err)
	}
	return nil
}

func showCreateVmRequest(logger log.DebugLogger) error {
	request, err := makeVmCreateRequest(logger)
	if err != nil {
		return err
	}
	return json.WriteWithIndent(os.Stdout, "    ", request.CreateVmRequest)
}
