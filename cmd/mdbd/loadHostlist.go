package main

import (
	"bufio"
	"io"
	"strings"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
)

func newHostlistGenerator(params makeGeneratorParams) (generator, error) {
	var requiredImage, plannedImage string
	if len(params.args) > 1 {
		requiredImage = params.args[1]
	}
	if len(params.args) > 2 {
		plannedImage = params.args[2]
	}
	return sourceGenerator{func(reader io.Reader, _datacentre string,
		logger log.Logger) (*mdbType, error) {
		return loadHostlist(reader, requiredImage, plannedImage, logger)
	},
		params.args[0]}, nil
}

func loadHostlist(reader io.Reader, requiredImage, plannedImage string,
	logger log.Logger) (*mdbType, error) {
	scanner := bufio.NewScanner(reader)
	var newMdb mdbType
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) > 0 {
			if fields[0][0] == '#' {
				continue
			}
			newMdb.Machines = append(newMdb.Machines, &mdb.Machine{
				Hostname:      fields[0],
				RequiredImage: requiredImage,
				PlannedImage:  plannedImage,
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return &newMdb, nil
}
