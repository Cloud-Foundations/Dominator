package main

import (
	"bufio"
	"encoding/gob"
	"errors"
	"os"
	"path"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
	"github.com/Cloud-Foundations/tricorder/go/tricorder"
	"github.com/Cloud-Foundations/tricorder/go/tricorder/units"
)

var (
	latencyBucketer         = tricorder.NewGeometricBucketer(0.1, 100e3)
	loadCpuTimeDistribution *tricorder.CumulativeDistribution
	loadTimeDistribution    *tricorder.CumulativeDistribution
)

type genericEncoder interface {
	Encode(v interface{}) error
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

func runDaemon(generators []generator, eventChannel <-chan struct{},
	mdbFileName string, hostnameRegex string,
	datacentre string, fetchInterval uint, updateFunc func(old, new *mdb.Mdb),
	logger log.DebugLogger, debug bool) {
	var prevMdb *mdb.Mdb
	var hostnameRE *regexp.Regexp
	var err error
	if hostnameRegex != ".*" {
		hostnameRE, err = regexp.Compile("^" + hostnameRegex)
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
		newMdb, err := loadFromAll(generators, datacentre, logger)
		if err != nil {
			logger.Println(err)
			continue
		}
		newMdb = selectHosts(newMdb, hostnameRE)
		sort.Sort(newMdb)
		if newMdbIsDifferent(prevMdb, newMdb) {
			updateFunc(prevMdb, newMdb)
			if err := writeMdb(newMdb, mdbFileName); err != nil {
				logger.Println(err)
			} else {
				if debug {
					logger.Printf("Wrote new MDB data, %d machines\n",
						len(newMdb.Machines))
				}
				prevMdb = newMdb
			}
		} else if debug {
			logger.Printf("Refreshed MDB data, same %d machines\n",
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

func loadFromAll(generators []generator, datacentre string,
	logger log.DebugLogger) (*mdb.Mdb, error) {
	machineMap := make(map[string]mdb.Machine)
	var variables map[string]string
	startTime := time.Now()
	var rusageStart, rusageStop syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &rusageStart)
	for _, gen := range generators {
		mdb, err := gen.Generate(datacentre, logger)
		if err != nil {
			return nil, err
		}
		for _, machine := range mdb.Machines {
			if oldMachine, ok := machineMap[machine.Hostname]; ok {
				oldMachine.UpdateFrom(machine)
				machineMap[machine.Hostname] = oldMachine
			} else {
				machineMap[machine.Hostname] = machine
			}
		}
		if vGen, ok := gen.(variablesGetter); ok {
			if _variables, err := vGen.GetVariables(); err != nil {
				return nil, err
			} else {
				variables = _variables
			}
		}
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

func selectHosts(inMdb *mdb.Mdb, hostnameRE *regexp.Regexp) *mdb.Mdb {
	if hostnameRE == nil {
		return inMdb
	}
	var outMdb mdb.Mdb
	for _, machine := range inMdb.Machines {
		if hostnameRE.MatchString(machine.Hostname) {
			outMdb.Machines = append(outMdb.Machines, machine)
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
