package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/triggers"
)

func diffTriggersInImagesSubcommand(args []string,
	logger log.DebugLogger) error {
	err := diffTriggersInImages(args[0], args[1], args[2])
	if err != nil {
		return fmt.Errorf("error diffing triggers: %s", err)
	}
	return nil
}

func diffTriggersInImages(tool, leftName, rightName string) error {
	leftTriggers, err := getTypedImageTriggers(leftName)
	if err != nil {
		return err
	}
	rightTriggers, err := getTypedImageTriggers(rightName)
	if err != nil {
		return err
	}
	leftFile, err := writeTriggersToTempfile(leftTriggers)
	if err != nil {
		return err
	}
	defer os.Remove(leftFile)
	rightFile, err := writeTriggersToTempfile(rightTriggers)
	if err != nil {
		return err
	}
	defer os.Remove(rightFile)
	cmd := exec.Command(tool, leftFile, rightFile)
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

func writeTriggersToTempfile(trig *triggers.Triggers) (string, error) {
	return writeToTempfile(func(writer io.Writer) error {
		return json.WriteWithIndent(writer, "    ", trig.Triggers)
	})
}
