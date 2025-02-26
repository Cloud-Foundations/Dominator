package scanner

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
)

func (fsh *FileSystemHistory) writeHtml(writer io.Writer) {
	fmt.Fprintf(writer, "Scan count: %d<br>\n", fsh.scanCount)
	fmt.Fprintf(writer, "Generation count: %d<br>\n", fsh.generationCount)
	if fsh.scanCount > 0 {
		fmt.Fprintf(writer, "Last scan completed: %s<br>\n", fsh.timeOfLastScan)
		fmt.Fprintf(writer, "Duration of last scan: %s<br>\n",
			fsh.durationOfLastScan)
		fsh.fileSystem.WriteHtml(writer)
		tmp := format.FormatBytes(uint64(float64(
			fsh.fileSystem.TotalDataBytes) / fsh.durationOfLastScan.Seconds()))
		fmt.Fprintf(writer, "Scan rate: %s/s<br>\n", tmp)
	}
	fmt.Fprintf(writer, "Duration of current scan: %s<br>\n",
		time.Since(fsh.timeOfLastScan))
	if fsh.generationCount > 0 {
		fmt.Fprintf(writer, "Last change: %s (%s ago)<br>\n",
			fsh.timeOfLastChange, time.Since(fsh.timeOfLastChange))
	}
	fmt.Fprintln(writer, `Show <a href="showScanFilter">scan filter</a><br>`)
}

func (fs *FileSystem) writeHtml(writer io.Writer) {
	fmt.Fprintf(writer, "Scanned: <a href=\"dumpFileSystem\">%s</a><br>\n",
		format.FormatBytes(fs.TotalDataBytes))
}

func (configuration *Configuration) writeHtml(writer io.Writer) {
	speed := "unknown"
	ctx := configuration.NetworkReaderContext
	if ctx.MaximumSpeed() > 0 {
		speed = fmt.Sprintf("%s/s (%d%% of %s/s)",
			format.FormatBytes(
				ctx.MaximumSpeed()*uint64(ctx.SpeedPercent())/100),
			ctx.SpeedPercent(), format.FormatBytes(ctx.MaximumSpeed()))
	}
	fmt.Fprintf(writer, "Network Speed: %s<br>\n", speed)
}

func (configuration *Configuration) showScanFilterHandler(
	w http.ResponseWriter, req *http.Request) {
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	fmt.Fprintln(writer, "<title>Scan filter</title>")
	fmt.Fprintln(writer, "<body>")
	fmt.Fprintln(writer, "<h3>")
	configuration.ScanFilter.WriteHtml(writer)
	fmt.Fprintln(writer, "</h3>")
	fmt.Fprintln(writer, "</body>")
}
