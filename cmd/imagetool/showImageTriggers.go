package main

import (
	"fmt"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func showImageTriggersSubcommand(args []string, logger log.DebugLogger) error {
	if err := showImageTriggers(args[0]); err != nil {
		return fmt.Errorf("error showing image triggers: %s", err)
	}
	return nil
}

func showImageTriggers(imageName string) error {
	trig, err := getTypedImageTriggers(imageName)
	if err != nil {
		return err
	}
	return json.WriteWithIndent(os.Stdout, "    ", trig.Triggers)
}
