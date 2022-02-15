package rpcd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/netspeed"
	"github.com/Cloud-Foundations/Dominator/lib/objectcache"
	objectclient "github.com/Cloud-Foundations/Dominator/lib/objectserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/rateio"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/sub"
)

const filePerms = syscall.S_IRUSR | syscall.S_IWUSR | syscall.S_IRGRP

var (
	exitOnFetchFailure = flag.Bool("exitOnFetchFailure", false,
		"If true, exit if there are fetch failures. For debugging only")
)

func (t *rpcType) Fetch(conn *srpc.Conn, request sub.FetchRequest,
	reply *sub.FetchResponse) error {
	if *readOnly {
		txt := "Fetch() rejected due to read-only mode"
		t.params.Logger.Println(txt)
		return errors.New(txt)
	}
	if err := t.getFetchLock(conn, request); err != nil {
		return err
	}
	if request.Wait {
		return t.fetchAndUnlock(conn, request, conn.Username())
	}
	go t.fetchAndUnlock(conn, request, conn.Username())
	return nil
}

func (t *rpcType) getFetchLock(conn *srpc.Conn,
	request sub.FetchRequest) error {
	t.rwLock.Lock()
	defer t.rwLock.Unlock()
	if err := t.getClientLock(conn, request.LockFor); err != nil {
		t.params.Logger.Printf("Error: %s\n", err)
		return err
	}
	if t.fetchInProgress {
		t.params.Logger.Println("Error: fetch already in progress")
		return errors.New("fetch already in progress")
	}
	if t.updateInProgress {
		t.params.Logger.Println("Error: update in progress")
		return errors.New("update in progress")
	}
	t.fetchInProgress = true
	return nil
}

func (t *rpcType) fetchAndUnlock(conn *srpc.Conn, request sub.FetchRequest,
	username string) error {
	err := t.doFetch(request, username)
	if err != nil && *exitOnFetchFailure {
		os.Exit(1)
	}
	t.rwLock.Lock()
	defer t.rwLock.Unlock()
	t.fetchInProgress = false
	t.lastFetchError = err
	if err := t.getClientLock(conn, request.LockFor); err != nil {
		return err
	}
	return err
}

func (t *rpcType) doFetch(request sub.FetchRequest, username string) error {
	objectServer := objectclient.NewObjectClient(request.ServerAddress)
	defer objectServer.Close()
	defer t.params.ScannerConfiguration.BoostCpuLimit(t.params.Logger)
	benchmark := false
	linkSpeed, haveLinkSpeed := netspeed.GetSpeedToAddress(
		request.ServerAddress)
	if haveLinkSpeed {
		t.logFetch(request, linkSpeed, username)
	} else {
		if t.params.NetworkReaderContext.MaximumSpeed() < 1 {
			benchmark = enoughBytesForBenchmark(objectServer, request)
			if benchmark {
				objectServer.SetExclusiveGetObjects(true)
				var suffix string
				if username != "" {
					suffix = " by " + username
				}
				t.params.Logger.Printf(
					"Fetch(%s) %d objects and benchmark speed%s\n",
					request.ServerAddress, len(request.Hashes), suffix)
			} else {
				t.logFetch(request, 0, username)
			}
		} else {
			t.logFetch(request, t.params.NetworkReaderContext.MaximumSpeed(),
				username)
		}
	}
	objectsReader, err := objectServer.GetObjects(request.Hashes)
	if err != nil {
		t.params.Logger.Printf("Error getting object reader: %s\n", err.Error())
		return err
	}
	defer objectsReader.Close()
	var totalLength uint64
	defer t.params.WorkdirGoroutine.Run(t.params.RescanObjectCacheFunction)
	timeStart := time.Now()
	for _, hash := range request.Hashes {
		length, reader, err := objectsReader.NextObject()
		if err != nil {
			t.params.Logger.Println(err)
			return err
		}
		r := io.Reader(reader)
		if haveLinkSpeed {
			if linkSpeed > 0 {
				r = rateio.NewReaderContext(linkSpeed,
					uint64(t.params.NetworkReaderContext.SpeedPercent()),
					&rateio.ReadMeasurer{}).NewReader(reader)
			}
		} else if !benchmark {
			r = t.params.NetworkReaderContext.NewReader(reader)
		}
		t.params.WorkdirGoroutine.Run(func() {
			err = readOne(t.config.ObjectsDirectoryName, hash, length, r)
		})
		reader.Close()
		if err != nil {
			t.params.Logger.Println(err)
			return err
		}
		totalLength += length
	}
	duration := time.Since(timeStart)
	speed := uint64(float64(totalLength) / duration.Seconds())
	if benchmark {
		file, err := os.Create(t.config.NetworkBenchmarkFilename)
		if err == nil {
			fmt.Fprintf(file, "%d\n", speed)
			file.Close()
		}
		t.params.NetworkReaderContext.InitialiseMaximumSpeed(speed)
	}
	t.params.Logger.Printf("Fetch() complete. Read: %s in %s (%s/s)\n",
		format.FormatBytes(totalLength), format.Duration(duration),
		format.FormatBytes(speed))
	return nil
}

func (t *rpcType) logFetch(request sub.FetchRequest, speed uint64,
	username string) {
	speedString := "unlimited speed"
	if speed > 0 {
		speedString = format.FormatBytes(
			speed*uint64(
				t.params.NetworkReaderContext.SpeedPercent())/100) + "/s"
	}
	var suffix string
	if username != "" {
		suffix = " by " + username
	}
	t.params.Logger.Printf("Fetch(%s) %d objects at %s%s\n",
		request.ServerAddress, len(request.Hashes), speedString, suffix)
}

func enoughBytesForBenchmark(objectServer *objectclient.ObjectClient,
	request sub.FetchRequest) bool {
	lengths, err := objectServer.CheckObjects(request.Hashes)
	if err != nil {
		return false
	}
	var totalLength uint64
	for _, length := range lengths {
		totalLength += length
	}
	if totalLength > 1024*1024*64 {
		return true
	}
	return false
}

func readOne(objectsDir string, hash hash.Hash, length uint64,
	reader io.Reader) error {
	filename := path.Join(objectsDir, objectcache.HashToFilename(hash))
	dirname := path.Dir(filename)
	if err := os.MkdirAll(dirname, syscall.S_IRWXU); err != nil {
		return err
	}
	return fsutil.CopyToFile(filename, filePerms, reader, length)
}
