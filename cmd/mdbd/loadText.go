package main

import (
	"bufio"
	"io"
	"strings"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
)

func newTextGenerator(params makeGeneratorParams) (generator, error) {
	return sourceGenerator{loadText, params.args[0]}, nil
}

func loadText(reader io.Reader, datacentre string, logger log.Logger) (
	*mdbType, error) {
	scanner := bufio.NewScanner(reader)
	var newMdb mdbType
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) > 0 {
			if fields[0][0] == '#' {
				continue
			}
			var machine mdb.Machine
			machine.Hostname = fields[0]
			if len(fields) > 1 {
				machine.RequiredImage = fields[1]
				if len(fields) > 2 {
					machine.PlannedImage = fields[2]
					if len(fields) > 3 && fields[3] == "true" {
						machine.DisableUpdates = true
					}
				}
			}
			newMdb.Machines = append(newMdb.Machines, &machine)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return &newMdb, nil
}
