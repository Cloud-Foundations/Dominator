package main

import (
	"fmt"
	"os"

	"github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
	mdbclient "github.com/Cloud-Foundations/Dominator/lib/mdb/client"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/text"
	"github.com/Cloud-Foundations/Dominator/lib/verstr"
)

func showComputedFileSubsSubcommand(args []string,
	logger log.DebugLogger) error {
	imageSClient, _ := getClients()
	mdbdSClient, err := dialMdbd()
	if err != nil {
		return err
	}
	cf := filesystem.ComputedFile{
		Filename: args[0],
		Source:   args[1],
	}
	if err := showComputedFileSubs(imageSClient, mdbdSClient, cf); err != nil {
		return fmt.Errorf("error showing subs with computed file: %s", err)
	}
	return nil
}

func showComputedFileSubs(imageSClient, mdbdSClient srpc.ClientI,
	computedFile filesystem.ComputedFile) error {
	// Get data from MDB.
	reply, err := mdbclient.GetMdb(mdbdSClient)
	if err != nil {
		return err
	}
	imageToMachinesMap := computeImageToMachinesMap(reply.Machines, false)
	machinesMap := make(map[*mdb.Machine]struct{})
	for imageName, imageMachines := range imageToMachinesMap {
		computedFiles, _, err := client.GetImageComputedFiles(imageSClient,
			imageName)
		if err != nil {
			return err
		}
		for _, imageComputedFile := range computedFiles {
			if imageComputedFile == computedFile {
				for _, machine := range imageMachines {
					machinesMap[machine] = struct{}{}
				}
				break
			}
		}
	}
	hostnames := make([]string, 0, len(machinesMap))
	hostnameToImageMap := make(map[string]string, len(machinesMap))
	for machine := range machinesMap {
		hostnames = append(hostnames, machine.Hostname)
		hostnameToImageMap[machine.Hostname] = machine.RequiredImage
	}
	verstr.Sort(hostnames)
	columnCollector := &text.ColumnCollector{}
	for _, hostname := range hostnames {
		columnCollector.AddField(hostname)
		columnCollector.AddField(hostnameToImageMap[hostname])
		columnCollector.CompleteLine()
	}
	return columnCollector.WriteLeftAligned(os.Stdout)
}
