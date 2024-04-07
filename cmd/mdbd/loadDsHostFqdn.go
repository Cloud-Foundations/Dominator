package main

import (
	"errors"
	"io"

	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
)

func newDsHostFqdnGenerator(params makeGeneratorParams) (generator, error) {
	return sourceGenerator{loadDsHostFqdn, params.args[0]}, nil
}

func loadDsHostFqdn(reader io.Reader, datacentre string, logger log.Logger) (
	*mdb.Mdb, error) {
	type machineType struct {
		Fqdn string
	}

	type dataCentreType map[string]machineType

	type inMdbType map[string]dataCentreType

	var inMdb inMdbType
	var outMdb mdb.Mdb
	if err := json.Read(reader, &inMdb); err != nil {
		return nil, errors.New("error decoding: " + err.Error())
	}
	for dsName, dataCentre := range inMdb {
		if datacentre != "" && dsName != datacentre {
			continue
		}
		for _, inMachine := range dataCentre {
			var outMachine mdb.Machine
			if inMachine.Fqdn != "" {
				outMachine.Hostname = inMachine.Fqdn
				outMdb.Machines = append(outMdb.Machines, outMachine)
			}
		}
	}
	return &outMdb, nil
}
