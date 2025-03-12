package main

import (
	"bufio"
	"fmt"
	"os"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func showImageSubcommand(args []string, logger log.DebugLogger) error {
	if err := showImage(args[0], logger); err != nil {
		return fmt.Errorf("error showing image: %s", err)
	}
	return nil
}

func showImage(image string, logger log.DebugLogger) error {
	fs, err := getTypedFileSystem(image)
	if err != nil {
		return err
	}
	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()
	startTime := time.Now()
	if err := fs.Listf(writer, listSelector, listFilter); err != nil {
		return err
	}
	logger.Debugf(0, "listing image took: %s\n",
		format.Duration(time.Since(startTime)))
	return nil
}
