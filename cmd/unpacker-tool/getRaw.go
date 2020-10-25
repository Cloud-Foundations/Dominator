package main

import (
	"fmt"
	"time"

	uclient "github.com/Cloud-Foundations/Dominator/imageunpacker/client"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

func getRawSubcommand(args []string, logger log.DebugLogger) error {
	if err := getRaw(getClient(), args[0], logger); err != nil {
		return fmt.Errorf("Error getting raw data: %s", err)
	}
	return nil
}

func getRaw(client *srpc.Client, streamName string,
	logger log.DebugLogger) error {
	if *filename == "" {
		return fmt.Errorf("no image filename specified")
	}
	reader, length, err := uclient.GetRaw(client, streamName)
	if err != nil {
		return err
	}
	defer reader.Close()
	startTime := time.Now()
	err = fsutil.CopyToFile(*filename, fsutil.PrivateFilePerms, reader, length)
	if err != nil {
		return err
	}
	timeTaken := time.Since(startTime)
	logger.Printf("downloaded %s in %s (%s/s)\n",
		format.FormatBytes(length), format.Duration(timeTaken),
		format.FormatBytes(uint64(float64(length)/timeTaken.Seconds())))
	return nil
}
