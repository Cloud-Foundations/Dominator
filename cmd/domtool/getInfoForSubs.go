package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	libjson "github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/dominator"
)

func getInfoForSubsSubcommand(args []string, logger log.DebugLogger) error {
	if err := getInfoForSubs(getClient()); err != nil {
		return fmt.Errorf("error getting info for subs: %s", err)
	}
	return nil
}

func getInfoForSubs(client *srpc.Client) error {
	hostnames, err := getSubsFromFile()
	if err != nil {
		return err
	}
	request := dominator.GetInfoForSubsRequest{
		Hostnames:       hostnames,
		StatusesToMatch: statusesToMatch,
		TagsToMatch:     tagsToMatch,
	}
	var reply dominator.GetInfoForSubsResponse
	if err := client.RequestReply("Dominator.GetInfoForSubs", request,
		&reply); err != nil {
		return err
	}
	if err := errors.New(reply.Error); err != nil {
		return err
	}
	libjson.WriteWithIndent(os.Stdout, "    ", reply.Subs)
	return nil
}

func getSubsFromFile() ([]string, error) {
	if *subsList == "" {
		return nil, nil
	}
	file, err := os.Open(*subsList)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	if filepath.Ext(*subsList) != ".json" {
		return fsutil.ReadLines(reader)
	}
	var data interface{}
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}
	entries, ok := data.([]interface{})
	if !ok {
		return nil, errors.New("non slice data")
	}
	if len(entries) < 1 {
		return nil, nil
	}
	hostnames := make([]string, 0, len(entries))
	for _, entry := range entries {
		if hostname, ok := entry.(string); ok {
			hostnames = append(hostnames, hostname)
		} else if mapEntry, ok := entry.(map[string]interface{}); !ok {
			return nil, fmt.Errorf("unsupported entry type: %T, value: %v",
				entry, entry)
		} else {
			if value, ok := mapEntry["Hostname"]; !ok {
				return nil, fmt.Errorf("map entry missing Hostname: %v",
					entry)
			} else if hostname, ok := value.(string); !ok {
				return nil, fmt.Errorf("Hostname not a string: %v",
					value)
			} else {
				hostnames = append(hostnames, hostname)
			}
		}
	}
	return hostnames, nil
}
