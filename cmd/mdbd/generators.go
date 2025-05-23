package main

import (
	"bufio"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/Cloud-Foundations/Dominator/lib/expand"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

type generatorInfo struct {
	args                []string
	driverName          string
	generator           generator
	mutex               sync.Mutex
	numFilteredMachines uint
	numRawMachines      uint
}

type generatorList struct {
	generatorInfos []*generatorInfo
	maxArgs        uint
}

type makeGeneratorParams struct {
	args         []string
	eventChannel chan<- struct{}
	logger       log.DebugLogger
	waitGroup    *sync.WaitGroup
}

type makeGeneratorFunc func(makeGeneratorParams) (generator, error)

type sourceDriverFunc func(reader io.Reader, datacentre string,
	logger log.Logger) (*mdbType, error)

// The generator interface generates an mdb from some source.
type generator interface {
	Generate(datacentre string, logger log.DebugLogger) (*mdbType, error)
}

// The variablesGetter interface gets variables from some source.
type variablesGetter interface {
	GetVariables() (map[string]string, error)
}

func setupGenerators(reader io.Reader, drivers []driver,
	params makeGeneratorParams,
	variables map[string]string) (*generatorList, error) {
	genList := &generatorList{}
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 1 || len(fields[0]) < 1 || fields[0][0] == '#' {
			continue
		}
		var args []string
		for _, arg := range fields[1:] {
			args = append(args, expand.Opportunistic(arg,
				func(name string) string {
					return variables[name]
				}))
		}
		genInfo := &generatorInfo{
			args:       args,
			driverName: fields[0],
		}
		if uint(len(genInfo.args)) > genList.maxArgs {
			genList.maxArgs = uint(len(genInfo.args))
		}
		var drv *driver
		for _, d := range drivers {
			if d.name == genInfo.driverName {
				drv = &d
				break
			}
		}
		if drv == nil {
			return nil, errors.New("unknown driver: " + genInfo.driverName)
		}
		if len(genInfo.args) < drv.minArgs {
			return nil,
				errors.New("insufficient arguments for: " + genInfo.driverName)
		}
		if drv.maxArgs >= 0 && len(genInfo.args) > drv.maxArgs {
			return nil,
				errors.New("too many arguments for: " + genInfo.driverName)
		}
		params.args = genInfo.args
		gen, err := drv.setupFunc(params)
		if err != nil {
			return nil, err
		}
		genInfo.generator = gen
		genList.generatorInfos = append(genList.generatorInfos, genInfo)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	// Pad generator fields for display.
	for _, genInfo := range genList.generatorInfos {
		for index := uint(len(genInfo.args)); index < genList.maxArgs; index++ {
			genInfo.args = append(genInfo.args, "")
		}
	}
	return genList, nil
}

// sourceGenerator implements the generator interface and generates an *mdbType
// from either a flat file or a URL.
type sourceGenerator struct {
	driverFunc sourceDriverFunc // Parses the data from URL or flat file.
	url        string           // The URL or path of the flat file.
}

func (s sourceGenerator) Generate(datacentre string, logger log.DebugLogger) (
	*mdbType, error) {
	return loadMdb(s.driverFunc, s.url, datacentre, logger)
}

func loadMdb(driverFunc sourceDriverFunc, url string, datacentre string,
	logger log.Logger) (*mdbType, error) {
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return loadHttpMdb(driverFunc, url, datacentre, logger)
	}
	file, err := os.Open(url)
	if err != nil {
		return nil, errors.New(("Error opening file " + err.Error()))
	}
	defer file.Close()
	return driverFunc(bufio.NewReader(file), datacentre, logger)
}

func loadHttpMdb(driverFunc sourceDriverFunc, url string, datacentre string,
	logger log.Logger) (*mdbType, error) {
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("HTTP get failed")
	}
	return driverFunc(response.Body, datacentre, logger)
}
