package unpacker

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path"
	"strconv"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	proto "github.com/Cloud-Foundations/Dominator/proto/imageunpacker"
)

var (
	exportImageTool = flag.String("exportImageTool",
		"/usr/local/etc/export-image", "Name of tool to export image")
	exportImageUsername = flag.String("exportImageUsername",
		"nobody", "Username to run as for export tool")
)

func (u *Unpacker) exportImage(streamName string,
	exportType string, exportDestination string) error {
	u.rwMutex.Lock()
	u.updateUsageTimeWithLock()
	streamInfo, err := u.setupStream(streamName)
	u.rwMutex.Unlock()
	defer u.updateUsageTime()
	if err != nil {
		return err
	}
	errorChannel := make(chan error)
	request := requestType{
		request:           requestExport,
		exportType:        exportType,
		exportDestination: exportDestination,
		errorChannel:      errorChannel,
	}
	streamInfo.requestChannel <- request
	return <-errorChannel
}

func (stream *streamManagerState) export(exportType string,
	exportDestination string) error {
	userInfo, err := user.Lookup(*exportImageUsername)
	if err != nil {
		return err
	}
	groupIds, err := userInfo.GroupIds()
	if err != nil {
		return err
	}
	if err := stream.getDevice(); err != nil {
		return err
	}
	stream.unpacker.rwMutex.RLock()
	device := stream.unpacker.pState.Devices[stream.streamInfo.DeviceId]
	stream.unpacker.rwMutex.RUnlock()
	if err := stream.unmount(); err != nil {
		return err
	}
	stream.streamInfo.status = proto.StatusStreamExporting
	defer func() {
		stream.streamInfo.status = proto.StatusStreamNotMounted
	}()
	deviceFile, err := os.Open(path.Join("/dev", device.DeviceName))
	if err != nil {
		stream.streamInfo.dualLogger.Println("Error exporting: %s", err)
		return fmt.Errorf("error exporting: %s", err)
	}
	defer deviceFile.Close()
	cmd := exec.Command(*exportImageTool, exportType, exportDestination)
	cmd.Stdin = deviceFile
	uid, err := strconv.ParseUint(userInfo.Uid, 10, 32)
	if err != nil {
		return err
	}
	gid, err := strconv.ParseUint(userInfo.Gid, 10, 32)
	if err != nil {
		return err
	}
	gids := make([]uint32, 0, len(groupIds))
	for _, groupId := range groupIds {
		gid, err := strconv.ParseUint(groupId, 10, 32)
		if err != nil {
			return err
		}
		gids = append(gids, uint32(gid))
	}
	creds := &syscall.Credential{
		Uid:    uint32(uid),
		Gid:    uint32(gid),
		Groups: gids,
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: creds}
	startTime := time.Now()
	output, err := cmd.CombinedOutput()
	if err != nil {
		stream.streamInfo.dualLogger.Printf("Error exporting: %s: %s\n",
			err, string(output))
		return fmt.Errorf("error exporting: %s: %s", err, output)
	}
	stream.streamInfo.dualLogger.Printf(
		"Exported(%s) type: %s dest: %s in %s\n",
		stream.streamName, exportType, exportDestination,
		format.Duration(time.Since(startTime)))
	return nil
}
