package main

import (
	"fmt"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func showImageMetadataSubcommand(args []string, logger log.DebugLogger) error {
	if err := showImageMetadata(args[0]); err != nil {
		return fmt.Errorf("error showing image metadata: %s", err)
	}
	return nil
}

func showImageMetadata(imageName string) error {
	if img, err := getTypedImageMetadata(imageName); err != nil {
		return err
	} else if img == nil {
		return fmt.Errorf("no image")
	} else {
		return json.WriteWithIndent(os.Stdout, "    ", img)
	}
}
