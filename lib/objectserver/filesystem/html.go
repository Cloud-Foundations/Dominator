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
	duplicatedBytes := objSrv.duplicatedBytes
	numObjects := uint64(len(objSrv.objects))
	numDuplicated := objSrv.numDuplicated
	numReferenced := objSrv.numReferenced
	numUnreferenced := objSrv.numUnreferenced
	referencedBytes := objSrv.referencedBytes
	totalBytes := objSrv.totalBytes
	unreferencedBytes := objSrv.unreferencedBytes
	objSrv.rwLock.RUnlock()
	referencedUtilisation := float64(referencedBytes) * 100 / float64(capacity)
	totalUtilisation := float64(totalBytes) * 100 / float64(capacity)
	unreferencedObjectsPercent := 0.0
	unreferencedUtilisation := float64(unreferencedBytes) * 100 /
		float64(capacity)
	if numObjects > 0 {
		unreferencedObjectsPercent =
			100.0 * float64(numUnreferenced) / float64(numObjects)
	}
	unreferencedBytesPercent := 0.0
	if totalBytes > 0 {
		unreferencedBytesPercent =
			100.0 * float64(unreferencedBytes) / float64(totalBytes)
	}
	fmt.Fprintf(writer,
		"Number of objects: %d, consuming %s (%.1f%% of FS which is %.1f%% full)<br>\n",
		numObjects, format.FormatBytes(totalBytes), totalUtilisation,
		utilisation)
	if numDuplicated > 0 {
		fmt.Fprintf(writer,
			"Number of referenced objects: %d (%d duplicates, %.3g*), consuming %s (%.1f%% of FS, %s dups, %.3g*)<br>\n",
			numReferenced, numDuplicated,
			float64(numDuplicated)/float64(numReferenced),
			format.FormatBytes(referencedBytes),
			referencedUtilisation,
			format.FormatBytes(duplicatedBytes),
			float64(duplicatedBytes)/float64(referencedBytes))
	}
	fmt.Fprintf(writer,
		"Number of unreferenced objects: %d (%.1f%%), consuming %s (%.1f%%, %.1f%% of FS)<br>\n",
		numUnreferenced, unreferencedObjectsPercent,
		format.FormatBytes(unreferencedBytes), unreferencedBytesPercent,
		unreferencedUtilisation)
	if numReferenced+numUnreferenced != numObjects {
		fmt.Fprintf(writer,
			"<font color=\"red\">Object accounting error: ref+unref:%d != total: %d</font><br>\n",
			numReferenced+numUnreferenced, numObjects)
	}
	if referencedBytes+unreferencedBytes != totalBytes {
		fmt.Fprintf(writer,
			"<font color=\"red\">Storage accounting error: ref+unref:%s != total: %s</font><br>\n",
			format.FormatBytes(referencedBytes+unreferencedBytes),
			format.FormatBytes(totalBytes))
	}
	writeHtmlBarAvailable(writer, referencedBytes, unreferencedBytes, capacity)
}

func writeHtmlBarAvailable(writer io.Writer,
	referenced, unreferenced, total uint64) {
	free := total - referenced - unreferenced
	barColour := "grey"
	leftBarWidth := float64(referenced) / float64(total)
	middleBarWidth := float64(unreferenced) / float64(total)
	rightBarWidth := float64(free) / float64(total)
	if free < total/100 {
		barColour = "orange"
	}
	fmt.Fprintln(writer,
		`<table border="1" style="border-collapse: collapse"><tr><td>`)
	fmt.Fprintln(writer, `  <table border="0" style="width:1000px"><tr>`)
	fmt.Fprintf(writer,
		"    <td style=\"width:%.1f%%;background-color:%s\">&nbsp;</td>\n",
		leftBarWidth*100, "blue")
	fmt.Fprintf(writer,
		"    <td style=\"width:%.1f%%;background-color:%s\">&nbsp;</td>\n",
		middleBarWidth*100, barColour)
	fmt.Fprintf(writer,
		"    <td style=\"width:%.1f%%;background-color:%s\">&nbsp;</td>\n",
		rightBarWidth*100, "white")
	fmt.Fprintln(writer, "  </tr></table>")
	fmt.Fprintln(writer, "</td></tr></table>")
}
