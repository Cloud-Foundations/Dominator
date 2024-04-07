package main

import (
	"errors"
	"io"

	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
)

func newCisGenerator(params makeGeneratorParams) (generator, error) {
	return sourceGenerator{loadCis, params.args[0]}, nil
}

func loadCis(reader io.Reader, datacentre string, logger log.Logger) (
	*mdb.Mdb, error) {

	type instanceMetadataType struct {
		RequiredImage  string `json:"required_image"`
		PlannedImage   string `json:"planned_image"`
		DisableUpdates bool   `json:"disable_updates"`
		OwnerGroup     string `json:"owner_group"`
	}

	type sourceType struct {
		HostName         string               `json:"host_name"`
		InstanceMetadata instanceMetadataType `json:"instance_metadata"`
		Fqdn             string
	}

	type hitType struct {
		Source sourceType `json:"_source"`
	}

	type hitListType struct {
		Hits []hitType
	}

	type inMdbType struct {
		Hits hitListType
	}

	var inMdb inMdbType
	var outMdb mdb.Mdb
	if err := json.Read(reader, &inMdb); err != nil {
		return nil, errors.New("error decoding: " + err.Error())
	}
	for _, hit := range inMdb.Hits.Hits {
		var outMachine mdb.Machine
		if hit.Source.Fqdn != "" {
			outMachine.Hostname = hit.Source.Fqdn
		} else {
			outMachine.Hostname = hit.Source.HostName
		}
		outMachine.RequiredImage = hit.Source.InstanceMetadata.RequiredImage
		outMachine.PlannedImage = hit.Source.InstanceMetadata.PlannedImage
		outMachine.DisableUpdates = hit.Source.InstanceMetadata.DisableUpdates
		outMachine.OwnerGroup = hit.Source.InstanceMetadata.OwnerGroup
		outMdb.Machines = append(outMdb.Machines, outMachine)
	}
	return &outMdb, nil
}
