package rpcd

import (
	"fmt"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/analysis"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
	"github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func (t *srpcType) GetObjectStatisticsForImages(conn *srpc.Conn,
	request imageserver.GetObjectStatisticsForImagesRequest,
	reply *imageserver.GetObjectStatisticsForImagesResponse) error {
	if err := t.getObjectStatisticsForImages(request, reply); err != nil {
		reply.Error = err.Error()
	}
	return nil
}

func (t *srpcType) getObjectStatisticsForImages(
	request imageserver.GetObjectStatisticsForImagesRequest,
	reply *imageserver.GetObjectStatisticsForImagesResponse) error {
	images, err := t.imageDataBase.GetImages(request.ImageNames,
		request.IgnoreMissing)
	if err != nil {
		return err
	}
	t.analysisGoroutine.Run(func() {
		var ruStart, ruStop wsyscall.Rusage
		wsyscall.Getrusage(wsyscall.RUSAGE_THREAD, &ruStart)
		fileSystems := make([]*filesystem.FileSystem, 0, len(images))
		for index, img := range images {
			if img == nil {
				continue
			}
			if img.FileSystem == nil {
				err = fmt.Errorf("No FileSystem for image: %s",
					request.ImageNames[index])
				return
			}
			fileSystems = append(fileSystems, img.FileSystem)
			reply.NumImages++
		}
		statistics, e := analysis.GetStatisticsForFileSystems(fileSystems)
		if e != nil {
			err = e
			return
		}
		reply.NumFileInodes = statistics.NumFileInodes
		reply.NumObjects = statistics.NumObjects
		reply.TotalFileInodeBytes = statistics.TotalFileInodeBytes
		reply.TotalObjectBytes = statistics.TotalObjectBytes
		if e := wsyscall.Getrusage(wsyscall.RUSAGE_THREAD, &ruStop); e == nil {
			reply.ComputationTime =
				time.Duration(ruStop.Utime.Sec)*time.Second +
					time.Duration(ruStop.Utime.Usec)*time.Microsecond -
					time.Duration(ruStart.Utime.Sec)*time.Second -
					time.Duration(ruStart.Utime.Usec)*time.Microsecond
		}
	})
	return err
}
