package main

import (
	"fmt"
	"os"

	"github.com/Cloud-Foundations/Dominator/imagebuilder/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func getDirectedGraphSubcommand(args []string, logger log.DebugLogger) error {
	if err := getDirectedGraph(logger); err != nil {
		return fmt.Errorf("error getting directed graph: %s", err)
	}
	return nil
}

func getDirectedGraph(logger log.Logger) error {
	srpcClient := getImaginatorClient()
	if graph, err := client.GetDirectedGraph(srpcClient); err != nil {
		return err
	} else {
		os.Stdout.Write(graph)
		if graph[len(graph)-1] != '\n' {
			fmt.Println()
		}
	}
	return nil
}
