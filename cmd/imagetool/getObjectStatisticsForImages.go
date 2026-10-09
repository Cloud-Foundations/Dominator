package main

import (
	"fmt"
	"time"

	"github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func getObjectStatisticsForImagesSubcommand(args []string,
	logger log.DebugLogger) error {
	imageClient, _ := getClients()
	err := getObjectStatisticsForImages(imageClient, args[0], logger)
	if err != nil {
		return fmt.Errorf("error getting object statistics for images: %s", err)
	}
	return nil
}

func getObjectStatisticsForImages(imageSClient srpc.ClientI, filename string,
	logger log.DebugLogger) error {
	imageNames, err := fsutil.LoadLines(filename)
	if err != nil {
		return err
	}
	startTime := time.Now()
	statistics, err := client.GetObjectStatisticsForImages(imageSClient,
		proto.GetObjectStatisticsForImagesRequest{
			ImageNames: imageNames,
		})
	if err != nil {
		return err
	}
	duration := time.Since(startTime)
	logger.Printf("%d images\n", statistics.NumImages)
	logger.Printf("%d files using %s\n",
		statistics.NumFileInodes,
		format.FormatBytes(statistics.TotalFileInodeBytes))
	logger.Printf("%d unique objects using %s, %.3g de-duplication factor\n",
		statistics.NumObjects,
		format.FormatBytes(statistics.TotalObjectBytes),
		float64(statistics.TotalFileInodeBytes)/
			float64(statistics.TotalObjectBytes),
	)
	if statistics.ComputationTime > 0 {
		logger.Printf("%s duration with %s computation time\n",
			format.Duration(duration),
			format.Duration(statistics.ComputationTime))
	} else {
		logger.Printf("%s duration\n", format.Duration(duration))
	}
	return nil
}
