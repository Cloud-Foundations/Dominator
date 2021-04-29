package main

import (
	"fmt"
	"os"

	"github.com/Cloud-Foundations/Dominator/imagebuilder/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	proto "github.com/Cloud-Foundations/Dominator/proto/imaginator"
)

func getDirectedGraphSubcommand(args []string, logger log.DebugLogger) error {
	if err := getDirectedGraph(logger); err != nil {
		return fmt.Errorf("error getting directed graph: %s", err)
	}
	return nil
}

func getDirectedGraph(logger log.Logger) error {
	srpcClient := getImaginatorClient()
	req := proto.GetDirectedGraphRequest{}
	if graph, err := client.GetDirectedGraph(srpcClient, req); err != nil {
		return err
	} else {
		os.Stdout.Write(graph)
		if graph[len(graph)-1] != '\n' {
			fmt.Println()
		}
	}
	return nil
}
