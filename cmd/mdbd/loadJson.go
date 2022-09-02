package main

import (
	"encoding/json"
	"io"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
)

func newJsonGenerator(params makeGeneratorParams) (generator, error) {
	return sourceGenerator{loadJson, params.args[0]}, nil
}

func loadJson(reader io.Reader, datacentre string, logger log.Logger) (
	*mdb.Mdb, error) {
	var newMdb mdb.Mdb
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&newMdb.Machines); err != nil {
		return nil, err
	}
	for index := range newMdb.Machines {
		extractPlainTags(&newMdb.Machines[index])
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
