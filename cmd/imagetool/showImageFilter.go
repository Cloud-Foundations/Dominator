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
	if err := filt.Write(os.Stdout); err != nil {
		return err
	}
	unoptimisedLines := filt.ListUnoptimised()
	if len(unoptimisedLines) > 0 {
		fmt.Fprintln(os.Stderr,
			"The following filter expressions are not optimised:")
		for _, line := range unoptimisedLines {
			if _, err := fmt.Fprintln(os.Stderr, line); err != nil {
				return err
			}
		}
	}
	return nil
}
