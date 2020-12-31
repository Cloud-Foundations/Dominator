package main

import (
	"fmt"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func showImageSubcommand(args []string, logger log.DebugLogger) error {
	if err := showImage(args[0]); err != nil {
		return fmt.Errorf("error showing image: %s", err)
	}
	return nil
}

func showImage(image string) error {
	fs, err := getTypedImage(image)
	if err != nil {
		return err
	}
	return fs.Listf(os.Stdout, listSelector, listFilter)
}
