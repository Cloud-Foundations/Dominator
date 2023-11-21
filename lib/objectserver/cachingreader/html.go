package cachingreader

import (
	"fmt"
	"io"

	"github.com/Cloud-Foundations/Dominator/lib/format"
)

func (objSrv *ObjectServer) writeHtml(writer io.Writer) {
	objSrv.rwLock.RLock()
	numObjects := len(objSrv.objects)
	stats := objSrv.getStats(false)
	objSrv.rwLock.RUnlock()
	fmt.Fprintf(writer,
		"Objectcache max: %s, total: %s (%d), cached: %s, in use: %s, downloading: %s<br>\n",
		format.FormatBytes(objSrv.maxCachedBytes),
		format.FormatBytes(stats.CachedBytes+stats.DownloadingBytes),
		numObjects,
		format.FormatBytes(stats.CachedBytes),
		format.FormatBytes(stats.CachedBytes-stats.LruBytes),
		format.FormatBytes(stats.DownloadingBytes))
}
