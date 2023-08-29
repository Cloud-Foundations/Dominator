package main

import (
	"fmt"
	"os"

	"github.com/Cloud-Foundations/Dominator/imagebuilder/client"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	proto "github.com/Cloud-Foundations/Dominator/proto/imaginator"
)

func getDependenciesSubcommand(args []string, logger log.DebugLogger) error {
	if err := getDependencies(logger); err != nil {
		return fmt.Errorf("error getting dependencies: %s", err)
	}
	return nil
}

func getDependencies(logger log.Logger) error {
	srpcClient := getImaginatorClient()
	req := proto.GetDependenciesRequest{}
	if result, err := client.GetDependencies(srpcClient, req); err != nil {
		return err
	} else {
		json.WriteWithIndent(os.Stdout, "    ", result)
	}
	return nil
}
