package main

import (
	"fmt"
	"io"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func getFileInImageSubcommand(args []string, logger log.DebugLogger) error {
	var outFileName string
	if len(args) > 2 {
		outFileName = args[2]
	}
	if err := getFileInImage(args[0], args[1], outFileName); err != nil {
		return fmt.Errorf("error getting file in image: %s", err)
	}
	return nil
}

func getFileInImage(imageName, imageFile, outFileName string) error {
	if reader, err := getTypedFileReader(imageName, imageFile); err != nil {
		return err
	} else {
		defer reader.Close()
		if outFileName == "" {
			_, err := io.Copy(os.Stdout, reader)
			return err
		} else {
			return fsutil.CopyToFile(outFileName, fsutil.PublicFilePerms,
				reader, 0)
		}
	}
}
