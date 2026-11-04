package main

import (
	"fmt"
	"math"
	"net"
	"sort"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

type fileLengthType struct {
	filename string
	length   uint64
}

func showVmVirtualiserLogFilesSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := showVmVirtualiserLogFiles(args[0], logger); err != nil {
		return fmt.Errorf("error showing VM virtualiser log files: %s", err)
	}
	return nil
}

func showVmVirtualiserLogFiles(vmHostname string,
	logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return showVmVirtualiserLogFilesOnHypervisor(hypervisor, vmIP, logger)
	}
}

func showVmVirtualiserLogFilesOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	filenames, lengths, err := hyperclient.ListVmVirtualiserLogFiles(client,
		ipAddr)
	if err != nil {
		return err
	}
	list := make([]fileLengthType, 0, len(filenames))
	var maxLength uint64
	for index, filename := range filenames {
		list = append(list,
			fileLengthType{filename: filename, length: lengths[index]})
		if lengths[index] > maxLength {
			maxLength = lengths[index]
		}
	}
	sort.Slice(list, func(left, right int) bool {
		return list[left].filename < list[right].filename
	})
	numDigits := uint(math.Ceil(math.Log10(float64(maxLength))))
	for _, entry := range list {
		fmt.Printf("%*d %s\n", numDigits, entry.length, entry.filename)
	}
	return nil
}
