package filesystem

import (
	"fmt"
	"io"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
)

func (objSrv *ObjectServer) writeHtml(writer io.Writer) {
	objSrv.lockWatcher.WriteHtml(writer, "ObjectServer: ")
	free, capacity, err := objSrv.getSpaceMetrics()
	if err != nil {
		fmt.Fprintln(writer, err)
		return
	}
	utilisation := float64(capacity-free) * 100 / float64(capacity)
	var totalBytes uint64
	startTime := time.Now()
	objSrv.rwLock.RLock()
	numObjects := len(objSrv.sizesMap)
	for _, size := range objSrv.sizesMap {
		totalBytes += size
	}
	objSrv.rwLock.RUnlock()
	fmt.Fprintf(writer,
		"Number of objects: %d, consuming %s (FS is %.1f%% full), computed in %s<br>\n",
		numObjects, format.FormatBytes(totalBytes), utilisation,
		format.Duration(time.Since(startTime)))
}
