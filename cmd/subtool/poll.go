package main

import (
	"encoding/gob"
	"fmt"
	"os"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/sub"
	"github.com/Cloud-Foundations/Dominator/sub/client"
)

func pollSubcommand(args []string, logger log.DebugLogger) error {
	var err error
	var srpcClient *srpc.Client
	for iter := 0; *numPolls < 0 || iter < *numPolls; iter++ {
		if iter > 0 {
			time.Sleep(time.Duration(*interval) * time.Second)
		}
		if srpcClient == nil {
			srpcClient = getSubClient(logger)
		}
		var request sub.PollRequest
		var reply sub.PollResponse
		request.ShortPollOnly = *shortPoll
		pollStartTime := time.Now()
		err = client.CallPoll(srpcClient, request, &reply)
		fmt.Printf("Poll duration: %s, ScanCount: %d, GenerationCount: %d\n",
			time.Since(pollStartTime), reply.ScanCount, reply.GenerationCount)
		if err != nil {
			logger.Fatalf("Error calling: %s\n", err)
		}
		if *newConnection {
			srpcClient.Close()
			srpcClient = nil
		}
		fs := reply.FileSystem
		if fs == nil {
			if !*shortPoll {
				fmt.Println("No FileSystem pointer")
			}
		} else {
			fs.RebuildInodePointers()
			if *debug {
				fs.List(os.Stdout)
			} else {
				fmt.Println(fs)
			}
			fmt.Printf("Num objects: %d\n", len(reply.ObjectCache))
			if *file != "" {
				f, err := os.Create(*file)
				if err != nil {
					logger.Fatalf("Error creating: %s: %s\n", *file, err)
				}
				encoder := gob.NewEncoder(f)
				encoder.Encode(fs)
				f.Close()
			}
		}
		if reply.LastSuccessfulImageName != "" {
			fmt.Printf("Last successful image: \"%s\"\n",
				reply.LastSuccessfulImageName)
		}
		if reply.FreeSpace != nil {
			fmt.Printf("Free space: %s\n", format.FormatBytes(*reply.FreeSpace))
		}
	}
	time.Sleep(time.Duration(*wait) * time.Second)
	return nil
}
