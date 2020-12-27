package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/imagepublishers/amipublisher"
	libjson "github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func deleteSubcommand(args []string, logger log.DebugLogger) error {
	if err := deleteResources(args, logger); err != nil {
		return fmt.Errorf("error deleting resources: %s", err)
	}
	return nil
}

func deleteResources(resultsFiles []string, logger log.DebugLogger) error {
	results := make([]amipublisher.Resource, 0)
	for _, resultsFile := range resultsFiles {
		fileResults := make([]amipublisher.Resource, 0)
		if err := libjson.ReadFromFile(resultsFile, &fileResults); err != nil {
			return err
		}
		results = append(results, fileResults...)
	}
	return amipublisher.DeleteResources(results, logger)
}
