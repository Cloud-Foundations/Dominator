package mdbd

import (
	"bufio"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"reflect"
	"sort"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	jsonwriter "github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/mdbserver"
)

func startMdbDaemon(config Config, params Params) <-chan *mdb.Mdb {
	mdbChannel := make(chan *mdb.Mdb, 1)
	if config.MdbServerHostname != "" && config.MdbServerPortNum > 0 {
		go serverWatchDaemon(config.MdbServerHostname, config.MdbServerPortNum,
			config.MdbFileName, mdbChannel, params.Logger)
	} else {
		go fileWatchDaemon(config.MdbFileName, mdbChannel, params.Logger)
	}
	return mdbChannel
}

type genericDecoder interface {
	Decode(v interface{}) error
}

func fileWatchDaemon(mdbFileName string, mdbChannel chan<- *mdb.Mdb,
	logger log.DebugLogger) {
	var lastMdb *mdb.Mdb
	for readCloser := range fsutil.WatchFile(mdbFileName, logger) {
		newMdb := loadFile(readCloser, mdbFileName, logger)
		readCloser.Close()
		if newMdb == nil {
			continue
		}
		compareStartTime := time.Now()
		if lastMdb == nil || !reflect.DeepEqual(lastMdb, newMdb) {
			if lastMdb != nil {
				mdbCompareTimeDistribution.Add(time.Since(compareStartTime))
			}
			mdbChannel <- newMdb
			lastMdb = newMdb
		} else {
			logger.Debugln(1, "MDB file contained no changes")
		}
	}
}

func serverWatchDaemon(mdbServerHostname string, mdbServerPortNum uint,
	mdbFileName string, mdbChannel chan<- *mdb.Mdb, logger log.DebugLogger) {
	var lastMdb *mdb.Mdb
	if mdbData := readFile(mdbFileName, logger); mdbData != nil {
		mdbChannel <- mdbData
		lastMdb = mdbData
	}
	address := fmt.Sprintf("%s:%d", mdbServerHostname, mdbServerPortNum)
	for ; ; time.Sleep(time.Second) {
		client, err := srpc.DialHTTP("tcp", address, time.Second*15)
		if err != nil {
			logger.Println(err)
			continue
		}
		conn, err := client.Call("MdbServer.GetMdbUpdates")
		if err != nil {
			logger.Println(err)
			client.Close()
			continue
		}
		for {
			var mdbUpdate mdbserver.MdbUpdate
			if err := conn.Decode(&mdbUpdate); err != nil {
				logger.Println(err)
				break
			} else {
				newMdb := processUpdate(lastMdb, mdbUpdate)
				sort.Sort(newMdb)
				compareStartTime := time.Now()
				if lastMdb == nil || !reflect.DeepEqual(lastMdb, newMdb) {
					if lastMdb != nil {
						mdbCompareTimeDistribution.Add(time.Since(
							compareStartTime))
					}
					mdbChannel <- newMdb
					lastMdb = newMdb
					if err := writeFile(mdbFileName, newMdb); err != nil {
						logger.Println(err)
					} else {
						logger.Debugf(0, "Wrote MDB data to: %s\n", mdbFileName)
					}
				} else {
					logger.Debugln(1, "MDB update made no changes")
				}
			}
		}
		conn.Close()
		client.Close()
	}
}

func loadFile(reader io.Reader, filename string, logger log.Logger) *mdb.Mdb {
	decoder := getDecoder(reader, filename)
	var mdb mdb.Mdb
	decodeStartTime := time.Now()
	if err := decoder.Decode(&mdb.Machines); err != nil {
		logger.Printf("Error decoding MDB data: %s\n", err)
		return nil
	}
	sortStartTime := time.Now()
	mdbDecodeTimeDistribution.Add(sortStartTime.Sub(decodeStartTime))
	sort.Sort(&mdb)
	mdbSortTimeDistribution.Add(time.Since(sortStartTime))
	return &mdb
}

func isGob(filename string) bool {
	switch path.Ext(filename) {
	case ".gob":
		return true
	default:
		return false
	}
}

func getDecoder(reader io.Reader, filename string) genericDecoder {
	if isGob(filename) {
		return gob.NewDecoder(reader)
	} else {
		return json.NewDecoder(reader)
	}
}

func processUpdate(oldMdb *mdb.Mdb, mdbUpdate mdbserver.MdbUpdate) *mdb.Mdb {
	newMdb := &mdb.Mdb{}
	if oldMdb == nil || len(oldMdb.Machines) < 1 {
		newMdb.Machines = mdbUpdate.MachinesToAdd
		return newMdb
	}
	newMachines := make(map[string]mdb.Machine)
	for _, machine := range oldMdb.Machines {
		newMachines[machine.Hostname] = machine
	}
	for _, machine := range mdbUpdate.MachinesToAdd {
		newMachines[machine.Hostname] = machine
	}
	for _, machine := range mdbUpdate.MachinesToUpdate {
		newMachines[machine.Hostname] = machine
	}
	for _, name := range mdbUpdate.MachinesToDelete {
		delete(newMachines, name)
	}
	newMdb.Machines = make([]mdb.Machine, 0, len(newMachines))
	for _, machine := range newMachines {
		newMdb.Machines = append(newMdb.Machines, machine)
	}
	return newMdb
}

func readFile(filename string, logger log.Logger) *mdb.Mdb {
	if filename != "" {
		if file, err := os.Open(filename); err == nil {
			mdbData := loadFile(file, filename, logger)
			file.Close()
			return mdbData
		}
	}
	return nil
}

func writeFile(filename string, mdbData *mdb.Mdb) error {
	if filename == "" {
		return nil
	}
	file, err := fsutil.CreateRenamingWriter(filename, fsutil.PublicFilePerms)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	defer writer.Flush()
	if isGob(filename) {
		return gob.NewEncoder(writer).Encode(mdbData.Machines)
	} else {
		return jsonwriter.WriteWithIndent(writer, "    ", mdbData.Machines)
	}
}
