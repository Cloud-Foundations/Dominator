package main

import (
	"io"
	"path"

	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
)

type jsonLoaderType struct {
	locationPrefix string
}

func newJsonGenerator(params makeGeneratorParams) (generator, error) {
	loader := jsonLoaderType{}
	if len(params.args) > 1 {
		loader.locationPrefix = params.args[1]
	}
	return sourceGenerator{loader.loadJson, params.args[0]}, nil
}

func (l jsonLoaderType) loadJson(reader io.Reader, datacentre string,
	logger log.Logger) (
	*mdb.Mdb, error) {
	var newMdb mdb.Mdb
	if err := json.Read(reader, &newMdb.Machines); err != nil {
		return nil, err
	}
	for index := range newMdb.Machines {
		machine := &newMdb.Machines[index]
		if l.locationPrefix != "" {
			machine.Location = path.Join(l.locationPrefix, machine.Location)
		}
		extractPlainTags(machine)
	}
	return &newMdb, nil
}

func extractPlainTags(machine *mdb.Machine) {
	for key, value := range machine.Tags {
		switch key {
		case "RequiredImage":
			machine.RequiredImage = value
		case "PlannedImage":
			machine.PlannedImage = value
		case "DisableUpdates":
			machine.DisableUpdates = true
		case "OwnerGroup":
			machine.OwnerGroup = value
		}
	}
}
