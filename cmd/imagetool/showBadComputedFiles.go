package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
	"github.com/Cloud-Foundations/Dominator/proto/mdbserver"
)

func showBadComputedFilesSubcommand(args []string,
	logger log.DebugLogger) error {
	imageSClient, _ := getClients()
	mdbdSClient, err := dialMdbd()
	if err != nil {
		return err
	}
	if err := showBadComputedFiles(imageSClient, mdbdSClient); err != nil {
		return fmt.Errorf("error showing subs with missing computed files: %s",
			err)
	}
	return nil
}

func showBadComputedFiles(imageSClient, mdbdSClient srpc.ClientI) error {
	// Get data from MDB.
	request := mdbserver.GetMdbRequest{}
	var reply mdbserver.GetMdbResponse
	err := mdbdSClient.RequestReply("MdbServer.GetMdb", request, &reply)
	if err != nil {
		return err
	}
	if err := errors.New(reply.Error); err != nil {
		return err
	}
	imageToMachinesMap := computeImageToMachinesMap(reply.Machines, true)
	// Compute mapping from image to computed files and list of sources.
	filegenSources := make(map[string]struct{})
	imageToComputedFiles := make(map[string][]filesystem.ComputedFile,
		len(imageToMachinesMap))
	for imageName := range imageToMachinesMap {
		computedFiles, _, err := client.GetImageComputedFiles(imageSClient,
			imageName)
		if err != nil {
			logger.Printf("error getting computed files for image: %s: %s\n",
				imageName, err)
			continue
		}
		for _, computedFile := range computedFiles {
			filegenSources[computedFile.Source] = struct{}{}
		}
		imageToComputedFiles[imageName] = computedFiles
	}
	// Compute mapping from file generator to map of available computed paths.
	filegenToPathnames := make(map[string]map[string]struct{})
	for source := range filegenSources {
		pathnames, err := listFileGenerators(source, logger)
		if err != nil {
			return err
		}
		filegenToPathnames[source] = stringutil.ConvertListToMap(pathnames,
			false)
	}
	// Loop over images, showing those with missing computed files and the
	// affected subs.
	for imageName, machines := range imageToMachinesMap {
		var missingComputedFiles []filesystem.ComputedFile
		for _, cf := range imageToComputedFiles[imageName] {
			if _, ok := filegenToPathnames[cf.Source][cf.Filename]; !ok {
				missingComputedFiles = append(missingComputedFiles, cf)
			}
		}
		if len(missingComputedFiles) > 0 {
			fmt.Printf("%s has missing computed files\n", imageName)
			fmt.Println("  missing computed files:")
			for _, cf := range missingComputedFiles {
				fmt.Printf("    %s:%s\n", cf.Filename, cf.Source)
			}
			fmt.Println("  affected subs:")
			for _, machine := range machines {
				fmt.Printf("    %s\n", machine.Hostname)
			}
		}
	}
	return nil
}
