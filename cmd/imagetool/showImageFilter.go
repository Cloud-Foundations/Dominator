package main

import (
	"fmt"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func showImageFilterSubcommand(args []string, logger log.DebugLogger) error {
	if err := showImageFilter(args[0]); err != nil {
		return fmt.Errorf("error showing image filter: %s", err)
	}
	return nil
}

func showImageFilter(imageName string) error {
	filt, err := getTypedImageFilter(imageName)
	if err != nil {
		return err
	}
	return filt.Write(os.Stdout)
}
