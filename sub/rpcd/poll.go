package rpcd

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/sub"
)

var (
	filename  string    = "/proc/uptime"
	startTime time.Time = time.Now()
)

func (t *rpcType) Poll(conn *srpc.Conn) error {
	defer conn.Flush()
	var request sub.PollRequest
	var response sub.PollResponse
	if err := conn.Decode(&request); err != nil {
		_, err = conn.WriteString(err.Error() + "\n")
		return err
	}
	if !request.ShortPollOnly && !conn.GetAuthInformation().HaveMethodAccess {
		_, e := conn.WriteString(srpc.ErrorAccessToMethodDenied.Error() + "\n")
		return e
	}
	if _, err := conn.WriteString("\n"); err != nil {
		return err
	}
	response.NetworkSpeed = t.params.NetworkReaderContext.MaximumSpeed()
	response.CurrentConfiguration = t.getConfiguration()
	t.rwLock.RLock()
	response.FetchInProgress = t.fetchInProgress
	response.UpdateInProgress = t.updateInProgress
	if t.lastFetchError != nil {
		response.LastFetchError = t.lastFetchError.Error()
	}
	if !t.updateInProgress {
		if t.lastUpdateError != nil {
			response.LastUpdateError = t.lastUpdateError.Error()
		}
		response.LastUpdateHadTriggerFailures = t.lastUpdateHadTriggerFailures
	}
	response.LastSuccessfulImageName = t.lastSuccessfulImageName
	response.LastNote = t.lastNote
	response.LockedByAnotherClient =
		t.getClientLock(conn, request.LockFor) != nil
	response.LockedUntil = t.lockedUntil
	response.FreeSpace = t.getFreeSpace()
	response.DisruptionState = t.disruptionState
	t.rwLock.RUnlock()
	response.StartTime = startTime
	response.PollTime = time.Now()
	response.ScanCount = t.params.FileSystemHistory.ScanCount()
	response.DurationOfLastScan =
		t.params.FileSystemHistory.DurationOfLastScan()
	response.GenerationCount = t.params.FileSystemHistory.GenerationCount()
	response.SystemUptime = t.getSystemUptime()
	fs := t.params.FileSystemHistory.FileSystem()
	if fs != nil &&
		!request.ShortPollOnly &&
		request.HaveGeneration != t.params.FileSystemHistory.GenerationCount() {
		response.FileSystemFollows = true
	}
	if err := conn.Encode(response); err != nil {
		return err
	}
	if response.FileSystemFollows {
		if err := fs.FileSystem.Encode(conn); err != nil {
			return err
		}
		if err := fs.ObjectCache.Encode(conn); err != nil {
			return err
		}
	}
	return nil
}

func (t *rpcType) getFreeSpace() *uint64 {
	rootDir := t.config.RootDirectoryName
	if fd, err := syscall.Open(rootDir, syscall.O_RDONLY, 0); err != nil {
		t.params.Logger.Printf("error opening: %s: %s", rootDir, err)
		return nil
	} else {
		defer syscall.Close(fd)
		var statbuf syscall.Statfs_t
		if err := syscall.Fstatfs(fd, &statbuf); err != nil {
			t.params.Logger.Printf("error getting file-system stats: %s\n", err)
			return nil
		}
		retval := uint64(statbuf.Bfree * uint64(statbuf.Bsize))
		return &retval
	}
}

func (t *rpcType) getSystemUptime() *time.Duration {
	if uptime, err := getSystemUptime(); err != nil {
		t.params.Logger.Printf("error getting system uptime: %s\n", err)
		return nil
	} else {
		return &uptime
	}
}

func getSystemUptime() (time.Duration, error) {
	file, err := os.Open(filename)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	var idleTime, upTime float64
	nScanned, err := fmt.Fscanf(file, "%f %f", &upTime, &idleTime)
	if err != nil {
		return 0, err
	}
	if nScanned < 2 {
		return 0, fmt.Errorf("only read %d values from %s", nScanned, filename)
	}
	return time.Duration(upTime * float64(time.Second)), nil
}
