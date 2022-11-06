package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func estimateImageUsageSubcommand(args []string, logger log.DebugLogger) error {
	if err := estimateImageUsage(args[0]); err != nil {
		return fmt.Errorf("error estimating image size: %s", err)
	}
	return nil
}

func estimateImageUsage(image string) error {
	fs, err := getTypedFileSystem(image)
	if err != nil {
		return err
	}
	_, err = fmt.Println(format.FormatBytes(fs.EstimateUsage(0)))
	return err
}
