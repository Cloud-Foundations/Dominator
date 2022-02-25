package main

import (
	"bufio"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

func getImageArchiveDataSubcommand(args []string,
	logger log.DebugLogger) error {
	err := getImageArchiveDataAndWrite(args[0], args[1])
	if err != nil {
		return fmt.Errorf("error getting image: %s", err)
	}
	return nil
}

func getImageArchiveDataAndWrite(name, outputFilename string) error {
	img, err := getImageMetadata(name)
	if err != nil {
		return err
	}
	img.Filter = nil
	img.Triggers = nil
	var encoder srpc.Encoder
	if outputFilename == "-" {
		e := json.NewEncoder(os.Stdout)
		e.SetIndent("", "    ")
		encoder = e
	} else {
		file, err := fsutil.CreateRenamingWriter(outputFilename,
			fsutil.PublicFilePerms)
		if err != nil {
			return err
		}
		defer file.Close()
		writer := bufio.NewWriter(file)
		defer writer.Flush()
		if filepath.Ext(outputFilename) == ".json" {
			e := json.NewEncoder(writer)
			e.SetIndent("", "    ")
			encoder = e
		} else {
			encoder = gob.NewEncoder(writer)
		}
	}
	return encoder.Encode(img)
}
