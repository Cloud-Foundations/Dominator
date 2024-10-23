package main

import (
	"bufio"
	"encoding/gob"
	"errors"
	"io"
	"os"
	"path"
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
	"github.com/Cloud-Foundations/Dominator/lib/verstr"
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

type differenceStatsType struct {
	added   uint
	changed uint
	deleted uint
}

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
	datacentre string, fetchInterval uint, pauseTable *pauseTableType,
	updateFunc func(old, new *mdbType), logger log.DebugLogger) {
	var prevMdb *mdbType
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
			getHostsExcludes(), getHostsIncludes(), pauseTable, logger)
		if err != nil {
			logger.Println(err)
			continue
		}
		sort.SliceStable(newMdb.Machines, func(i, j int) bool {
			return verstr.Less(newMdb.Machines[i].Hostname,
				newMdb.Machines[j].Hostname)
		})
		stats := newMdbIsDifferent(prevMdb, newMdb)
		if stats.added < 1 && stats.changed < 1 && stats.deleted < 1 {
			logger.Debugf(1, "Refreshed MDB data, same %d machines\n",
				len(newMdb.Machines))
			continue
		}
		updateFunc(prevMdb, newMdb)
		if err := writeMdb(newMdb, mdbFileName); err != nil {
			logger.Println(err)
		} else {
			if prevMdb == nil {
				logger.Printf("Wrote initial MDB data, %d machines\n",
					len(newMdb.Machines))
			} else {
				logger.Debugf(0,
					"Wrote new MDB data, %d new machines, %d removed, %d changed\n",
					stats.added, stats.deleted, stats.changed)
			}
		}
		prevMdb = newMdb
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
	pauseTable *pauseTableType, logger log.DebugLogger) (*mdbType, error) {
	machineMap := make(map[string]*mdb.Machine)
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
				oldMachine.UpdateFrom(*machine)
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
	newMdb := mdbType{
		table: make(map[string]*mdb.Machine),
	}
	pauseTable.mutex.RLock()
	for _, machine := range machineMap {
		processMachine(machine, pauseTable, variables)
		newMdb.Machines = append(newMdb.Machines, machine)
		newMdb.table[machine.Hostname] = machine
	}
	pauseTable.mutex.RUnlock()
	syscall.Getrusage(syscall.RUSAGE_SELF, &rusageStop)
	loadTimeDistribution.Add(time.Since(startTime))
	loadCpuTimeDistribution.Add(time.Duration(
		rusageStop.Utime.Sec)*time.Second +
		time.Duration(rusageStop.Utime.Usec)*time.Microsecond -
		time.Duration(rusageStart.Utime.Sec)*time.Second -
		time.Duration(rusageStart.Utime.Usec)*time.Microsecond)
	return &newMdb, nil
}

func processMachine(machine *mdb.Machine, pauseTable *pauseTableType,
	variables map[string]string) {
	if !machine.DisableUpdates {
		if pauseData, ok := pauseTable.Machines[machine.Hostname]; ok {
			if time.Until(pauseData.Until) > 0 {
				machine.DisableUpdates = true
			}
		}
	}
	if len(variables) > 0 {
		machine.RequiredImage = processValue(machine.RequiredImage, variables)
		machine.PlannedImage = processValue(machine.PlannedImage, variables)
		machine.Tags = machine.Tags.Copy()
		for key, value := range machine.Tags {
			machine.Tags[key] = processValue(value, variables)
		}
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

func selectHosts(inMdb *mdbType, hostnameRE stringMatcher,
	hostsExcludeMap, hostsIncludeMap map[string]struct{}) *mdbType {
	if hostnameRE == nil &&
		len(hostsExcludeMap) < 1 &&
		len(hostsIncludeMap) < 1 {
		return inMdb
	}
	outMdb := mdbType{
		table: make(map[string]*mdb.Machine),
	}
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
			outMdb.table[machine.Hostname] = machine
		} else {
			if hostnameRE.MatchString(machine.Hostname) {
				outMdb.Machines = append(outMdb.Machines, machine)
				outMdb.table[machine.Hostname] = machine
			}
		}
	}
	return &outMdb
}

func newMdbIsDifferent(prevMdb, newMdb *mdbType) *differenceStatsType {
	if prevMdb == nil {
		return &differenceStatsType{added: uint(len(newMdb.Machines))}
	}
	var differenceStats differenceStatsType
	var numUnchanged uint
	for _, newMachine := range newMdb.Machines {
		if prevMachine, ok := prevMdb.table[newMachine.Hostname]; !ok {
			differenceStats.added++
		} else if prevMachine.Compare(*newMachine) {
			numUnchanged++
		} else {
			differenceStats.changed++
		}
	}
	differenceStats.deleted = uint(len(prevMdb.Machines)) -
		differenceStats.changed - numUnchanged
	return &differenceStats
}

func writeMdb(mdb *mdbType, mdbFileName string) error {
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
		if err := json.WriteWithIndent(writer, "    ", mdb.Machines); err != nil {
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
