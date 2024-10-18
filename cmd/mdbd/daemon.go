package main

import (
	"bufio"
	"encoding/gob"
	"errors"
	"io"
	"os"
	"path"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/configwatch"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
	"github.com/Cloud-Foundations/tricorder/go/tricorder"
	"github.com/Cloud-Foundations/tricorder/go/tricorder/units"
)

var (
	latencyBucketer         = tricorder.NewGeometricBucketer(0.1, 100e3)
	loadCpuTimeDistribution *tricorder.CumulativeDistribution
	loadTimeDistribution    *tricorder.CumulativeDistribution

	hostsExcludeMapMutex sync.RWMutex
	hostsExcludeMap      map[string]struct{}

	hostsIncludeMapMutex sync.RWMutex
	hostsIncludeMap      map[string]struct{}
)

type genericEncoder interface {
	Encode(v interface{}) error
}

type invertedRegexp struct {
	stringMatcher
}

type stringMatcher interface {
	MatchString(string) bool
}

func init() {
	loadCpuTimeDistribution = latencyBucketer.NewCumulativeDistribution()
	if err := tricorder.RegisterMetric("/load-cpu-time", loadCpuTimeDistribution,
		units.Millisecond, "load CPU time durations"); err != nil {
		panic(err)
	}
	loadTimeDistribution = latencyBucketer.NewCumulativeDistribution()
	if err := tricorder.RegisterMetric("/load-time", loadTimeDistribution,
		units.Millisecond, "load durations"); err != nil {
		panic(err)
	}
}

func runDaemon(generators *generatorList, eventChannel <-chan struct{},
	mdbFileName string, hostnameRegex string,
	datacentre string, fetchInterval uint, updateFunc func(old, new *mdb.Mdb),
	logger log.DebugLogger) {
	var prevMdb *mdb.Mdb
	var hostnameRE stringMatcher
	if hostnameRegex != ".*" {
		var err error
		var re *regexp.Regexp
		if strings.HasPrefix(hostnameRegex, "!") {
			re, err = regexp.Compile("^" + hostnameRegex[1:])
			hostnameRE = &invertedRegexp{re}
		} else {
			re, err = regexp.Compile("^" + hostnameRegex)
			hostnameRE = re
		}
		if err != nil {
			logger.Println(err)
			os.Exit(1)
		}
	}
	var cycleStopTime time.Time
	fetchIntervalDuration := time.Duration(fetchInterval) * time.Second
	intervalTimer := time.NewTimer(fetchIntervalDuration)
	for ; ; sleepUntil(eventChannel, intervalTimer, cycleStopTime) {
		cycleStopTime = time.Now().Add(fetchIntervalDuration)
		newMdb, err := loadFromAll(generators, datacentre, hostnameRE,
			getHostsExcludes(), getHostsIncludes(), logger)
		if err != nil {
			logger.Println(err)
			continue
		}
		sort.Sort(newMdb)
		if newMdbIsDifferent(prevMdb, newMdb) {
			updateFunc(prevMdb, newMdb)
			if err := writeMdb(newMdb, mdbFileName); err != nil {
				logger.Println(err)
			} else {
				if prevMdb == nil {
					logger.Printf("Wrote initial MDB data, %d machines\n",
						len(newMdb.Machines))
				} else {
					logger.Debugf(0, "Wrote new MDB data, %d machines\n",
						len(newMdb.Machines))
				}
				prevMdb = newMdb
			}
		} else {
			logger.Debugf(1, "Refreshed MDB data, same %d machines\n",
				len(newMdb.Machines))
		}
	}
}

func sleepUntil(eventChannel <-chan struct{}, intervalTimer *time.Timer,
	wakeTime time.Time) {
	runtime.GC() // An opportune time to take out the garbage.
	sleepTime := wakeTime.Sub(time.Now())
	if sleepTime < time.Second {
		sleepTime = time.Second
	}
	intervalTimer.Reset(sleepTime)
	select {
	case <-eventChannel:
	case <-intervalTimer.C:
	}
}

func loadFromAll(generators *generatorList, datacentre string,
	hostnameRE stringMatcher,
	hostsExcludeMap, hostsIncludeMap map[string]struct{},
	logger log.DebugLogger) (*mdb.Mdb, error) {
	machineMap := make(map[string]mdb.Machine)
	var variables map[string]string
	startTime := time.Now()
	var rusageStart, rusageStop syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &rusageStart)
	for _, genInfo := range generators.generatorInfos {
		mdb, err := genInfo.generator.Generate(datacentre, logger)
		if err != nil {
			return nil, err
		}
		numRawMachines := uint(len(mdb.Machines))
		mdb = selectHosts(mdb, hostnameRE, hostsExcludeMap, hostsIncludeMap)
		numFilteredMachines := uint(len(mdb.Machines))
		for _, machine := range mdb.Machines {
			if machine.Hostname == "" {
				machine.Hostname = machine.IpAddress
			}
			if machine.Hostname == "" {
				logger.Printf(
					"ignoring machine with no Hostname or IpAddress: %v\n",
					machine)
				continue
			}
			machine.DataSourceType = genInfo.driverName
			if oldMachine, ok := machineMap[machine.Hostname]; ok {
				oldMachine.UpdateFrom(machine)
				machineMap[machine.Hostname] = oldMachine
			} else {
				machineMap[machine.Hostname] = machine
			}
		}
		if vGen, ok := genInfo.generator.(variablesGetter); ok {
			if _variables, err := vGen.GetVariables(); err != nil {
				return nil, err
			} else {
				variables = _variables
			}
		}
		genInfo.mutex.Lock()
		genInfo.numFilteredMachines = numFilteredMachines
		genInfo.numRawMachines = numRawMachines
		genInfo.mutex.Unlock()
	}
	var newMdb mdb.Mdb
	for _, machine := range machineMap {
		processMachine(&machine, variables)
		newMdb.Machines = append(newMdb.Machines, machine)
	}
	syscall.Getrusage(syscall.RUSAGE_SELF, &rusageStop)
	loadTimeDistribution.Add(time.Since(startTime))
	loadCpuTimeDistribution.Add(time.Duration(
		rusageStop.Utime.Sec)*time.Second +
		time.Duration(rusageStop.Utime.Usec)*time.Microsecond -
		time.Duration(rusageStart.Utime.Sec)*time.Second -
		time.Duration(rusageStart.Utime.Usec)*time.Microsecond)
	return &newMdb, nil
}

func processMachine(machine *mdb.Machine, variables map[string]string) {
	if len(variables) < 1 {
		return
	}
	machine.RequiredImage = processValue(machine.RequiredImage, variables)
	machine.PlannedImage = processValue(machine.PlannedImage, variables)
	machine.Tags = machine.Tags.Copy()
	for key, value := range machine.Tags {
		machine.Tags[key] = processValue(value, variables)
	}
}

func processValue(value string, variables map[string]string) string {
	if len(value) < 2 {
		return value
	}
	if value[0] == '$' {
		if newValue, ok := variables[value[1:]]; ok {
			return newValue
		}
	}
	return value
}

func selectHosts(inMdb *mdb.Mdb, hostnameRE stringMatcher,
	hostsExcludeMap, hostsIncludeMap map[string]struct{}) *mdb.Mdb {
	if hostnameRE == nil &&
		len(hostsExcludeMap) < 1 &&
		len(hostsIncludeMap) < 1 {
		return inMdb
	}
	var outMdb mdb.Mdb
	for _, machine := range inMdb.Machines {
		if _, exclude := hostsExcludeMap[machine.Hostname]; exclude {
			continue
		}
		if len(hostsIncludeMap) > 0 && machine.Hostname != "" {
			if _, include := hostsIncludeMap[machine.Hostname]; !include {
				continue
			}
		}
		if hostnameRE == nil {
			outMdb.Machines = append(outMdb.Machines, machine)
		} else {
			if hostnameRE.MatchString(machine.Hostname) {
				outMdb.Machines = append(outMdb.Machines, machine)
			}
		}
	}
	return &outMdb
}

func newMdbIsDifferent(prevMdb, newMdb *mdb.Mdb) bool {
	return !reflect.DeepEqual(prevMdb, newMdb)
}

func writeMdb(mdb *mdb.Mdb, mdbFileName string) error {
	tmpFileName := mdbFileName + "~"
	file, err := os.Create(tmpFileName)
	if err != nil {
		return errors.New("error opening file " + err.Error())
	}
	defer os.Remove(tmpFileName)
	defer file.Close()
	writer := bufio.NewWriter(file)
	switch path.Ext(mdbFileName) {
	case ".gob":
		if err := gob.NewEncoder(writer).Encode(mdb.Machines); err != nil {
			return err
		}
	default:
		if err := json.WriteWithIndent(writer, "    ",
			mdb.Machines); err != nil {
			return err
		}
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	return os.Rename(tmpFileName, mdbFileName)
}

func getHostsExcludes() map[string]struct{} {
	hostsExcludeMapMutex.RLock()
	hostsMap := hostsExcludeMap
	hostsExcludeMapMutex.RUnlock()
	return hostsMap
}

func getHostsIncludes() map[string]struct{} {
	hostsIncludeMapMutex.RLock()
	hostsMap := hostsIncludeMap
	hostsIncludeMapMutex.RUnlock()
	return hostsMap
}

func hostsFilterReader(dataChannel <-chan interface{},
	eventChannel chan<- struct{}, waitGroup *sync.WaitGroup, lock sync.RWMutex,
	hostsMapPtr *map[string]struct{}) {
	for data := range dataChannel {
		lines := data.([]string)
		hostsMap := stringutil.ConvertListToMap(lines, false)
		lock.Lock()
		*hostsMapPtr = hostsMap
		lock.Unlock()
		if waitGroup != nil {
			waitGroup.Done()
			waitGroup = nil
		}
		eventChannel <- struct{}{}
	}
}

func startHostsExcludeReader(filename string, eventChannel chan<- struct{},
	waitGroup *sync.WaitGroup, logger log.DebugLogger) {
	startHostsFilterReader(filename, eventChannel, waitGroup,
		hostsExcludeMapMutex, &hostsExcludeMap, logger)
}

func startHostsFilterReader(filename string, eventChannel chan<- struct{},
	waitGroup *sync.WaitGroup, lock sync.RWMutex,
	hostsMapPtr *map[string]struct{}, logger log.DebugLogger) {
	if filename == "" {
		return
	}
	dataChannel, err := configwatch.Watch(filename,
		time.Minute, func(reader io.Reader) (interface{}, error) {
			return fsutil.ReadLines(reader)
		}, logger)
	if err != nil {
		showErrorAndDie(err)
	}
	waitGroup.Add(1)
	go hostsFilterReader(dataChannel, eventChannel, waitGroup, lock,
		hostsMapPtr)
}

func startHostsIncludeReader(filename string, eventChannel chan<- struct{},
	waitGroup *sync.WaitGroup, logger log.DebugLogger) {
	startHostsFilterReader(filename, eventChannel, waitGroup,
		hostsIncludeMapMutex, &hostsIncludeMap, logger)
}

func (ir *invertedRegexp) MatchString(s string) bool {
	return !ir.stringMatcher.MatchString(s)
}
