package filesystem

import (
	"fmt"
	"io"

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
	objSrv.rwLock.RLock()
	numObjects := len(objSrv.objects)
	numUnreferencedObjects := objSrv.numUnreferenced
	totalBytes := objSrv.totalBytes
	unreferencedBytes := objSrv.unreferencedBytes
	objSrv.rwLock.RUnlock()
	unreferencedObjectsPercent := 0.0
	if numObjects > 0 {
		unreferencedObjectsPercent =
			100.0 * float64(numUnreferencedObjects) / float64(numObjects)
	}
	unreferencedBytesPercent := 0.0
	if totalBytes > 0 {
		unreferencedBytesPercent =
			100.0 * float64(unreferencedBytes) / float64(totalBytes)
	}
	fmt.Fprintf(writer,
		"Number of objects: %d, consuming %s (FS is %.1f%% full)<br>\n",
		numObjects, format.FormatBytes(totalBytes), utilisation)
	fmt.Fprintf(writer,
		"Number of unreferenced objects: %d (%.1f%%), consuming %s (%.1f%%)<br>\n",
		numUnreferencedObjects, unreferencedObjectsPercent,
		format.FormatBytes(unreferencedBytes), unreferencedBytesPercent)
}
