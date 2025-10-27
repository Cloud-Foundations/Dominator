package html

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

type allCpuStats struct {
	self     cpuStats
	children cpuStats
}

type cpuStats struct {
	realTime time.Time
	userTime time.Duration
	sysTime  time.Duration
}

var (
	startCpuStats *allCpuStats = getCpuStats()
	lastCpuStats  *allCpuStats = startCpuStats
)

func handleFunc(serveMux *http.ServeMux, pattern string,
	handler func(w http.ResponseWriter, req *http.Request)) {
	serveMux.HandleFunc(pattern,
		func(w http.ResponseWriter, req *http.Request) {
			SetSecurityHeaders(w) // Compliance checkbox.
			handler(w, req)
		})
}

func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1")
	w.Header().Set("Content-Security-Policy",
		"default-src 'self' ;style-src 'self' 'unsafe-inline'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}

func writeCpuStats(writer io.Writer, prefix string, start, current *cpuStats) {
	userCpuTime := current.userTime - start.userTime
	sysCpuTime := current.sysTime - start.sysTime
	realTime := current.realTime.Sub(start.realTime)
	cpuTime := userCpuTime + sysCpuTime
	fmt.Fprintf(writer,
		"    <td>%s CPU Time: %.1f%% (User: %s Sys: %s)</td>\n",
		prefix, float64(cpuTime*100)/float64(realTime), userCpuTime, sysCpuTime)
}

func writeHeader(writer io.Writer, req *http.Request, noGC bool) {
	currentCpuStats := getCpuStats()
	fmt.Fprintln(writer,
		`<table border="1" bordercolor=#e0e0e0 style="border-collapse: collapse">`)
	fmt.Fprintf(writer, "  <tr>\n")
	fmt.Fprintf(writer, "    <td>Start time: %s</td>\n",
		startCpuStats.self.realTime.Format(format.TimeFormatSeconds))
	uptime := currentCpuStats.self.realTime.Sub(startCpuStats.self.realTime)
	uptime += time.Millisecond * 50
	uptime = (uptime / time.Millisecond / 100) * time.Millisecond * 100
	fmt.Fprintf(writer, "    <td>Uptime: %s</td>\n", format.Duration(uptime))
	fmt.Fprintf(writer, "  </tr>\n")
	fmt.Fprintf(writer, "  <tr>\n")
	writeCpuStats(writer, "Process Total",
		&startCpuStats.self, &currentCpuStats.self)
	writeCpuStats(writer, "Recent", &lastCpuStats.self, &currentCpuStats.self)
	fmt.Fprintf(writer, "  </tr>\n")
	fmt.Fprintf(writer, "  <tr>\n")
	writeCpuStats(writer, "Subprocess Total",
		&startCpuStats.children, &currentCpuStats.children)
	writeCpuStats(writer, "Recent",
		&lastCpuStats.children, &currentCpuStats.children)
	fmt.Fprintf(writer, "  </tr>\n")
	fmt.Fprintf(writer, "  <tr>\n")
	lastCpuStats = currentCpuStats
	var memStatsBeforeGC runtime.MemStats
	runtime.ReadMemStats(&memStatsBeforeGC)
	if noGC {
		fmt.Fprintf(writer, "    <td>Allocated memory: %s</td>\n",
			format.FormatBytes(memStatsBeforeGC.Alloc))
		fmt.Fprintf(writer, "    <td>System memory: %s</td>\n",
			format.FormatBytes(
				memStatsBeforeGC.Sys-memStatsBeforeGC.HeapReleased))
	} else {
		var memStatsAfterGC runtime.MemStats
		startTime := time.Now()
		runtime.GC()
		runtime.ReadMemStats(&memStatsAfterGC)
		fmt.Fprintf(writer, "    <td>Allocated memory: %s (%s after GC, took %s)</td>\n",
			format.FormatBytes(memStatsBeforeGC.Alloc),
			format.FormatBytes(memStatsAfterGC.Alloc),
			format.Duration(time.Since(startTime)))
		fmt.Fprintf(writer, "    <td>System memory: %s (%s after GC)</td>\n",
			format.FormatBytes(
				memStatsBeforeGC.Sys-memStatsBeforeGC.HeapReleased),
			format.FormatBytes(
				memStatsAfterGC.Sys-memStatsAfterGC.HeapReleased))
	}
	fmt.Fprintf(writer, "  </tr>\n")
	if hostname, err := os.Hostname(); err != nil {
		fmt.Fprintf(writer, "    <td>Error getting hostname: %s</td>\n", err)
	} else {
		fmt.Fprintf(writer, "    <td>Hostname: %s</td>\n", hostname)
	}
	fmt.Fprintln(writer, "    <td></td>")
	fmt.Fprintf(writer, "</table>\n")
	fmt.Fprintln(writer, "Raw <a href=\"metrics\">metrics</a><br>")
	if req != nil {
		protocol := "http"
		if req.TLS != nil {
			protocol = "https"
		}
		host := strings.Split(req.Host, ":")[0]
		fmt.Fprintf(writer,
			"Local <a href=\"%s://%s:6910/\">system health agent</a>",
			protocol, host)
	}
}

func getCpuStats() *allCpuStats {
	myUserTime, mySysTime, err := getRusage(wsyscall.RUSAGE_SELF)
	if err != nil {
		return nil
	}
	childUserTime, childSysTime, err := getRusage(wsyscall.RUSAGE_CHILDREN)
	if err != nil {
		return nil
	}
	now := time.Now()
	return &allCpuStats{
		self: cpuStats{
			realTime: now,
			userTime: myUserTime,
			sysTime:  mySysTime,
		},
		children: cpuStats{
			realTime: now,
			userTime: childUserTime,
			sysTime:  childSysTime,
		},
	}
}

// getRusage returns the user and system time used.
func getRusage(who int) (time.Duration, time.Duration, error) {
	var rusage wsyscall.Rusage
	if err := wsyscall.Getrusage(who, &rusage); err != nil {
		return 0, 0, err
	}
	return timevalToTime(rusage.Utime), timevalToTime(rusage.Stime), nil
}

func timevalToTime(timeval wsyscall.Timeval) time.Duration {
	return time.Duration(timeval.Sec)*time.Second +
		time.Duration(timeval.Usec)*time.Microsecond
}
