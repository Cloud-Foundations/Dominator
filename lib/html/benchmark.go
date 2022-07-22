package html

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

func createBenchmarkData() (*BenchmarkData, error) {
	runtime.LockOSThread()
	bd := BenchmarkData{}
	err := wsyscall.Getrusage(wsyscall.RUSAGE_THREAD, &bd.startRusage)
	if err != nil {
		return nil, err
	}
	bd.startTime = time.Now()
	return &bd, nil
}

func (bd *BenchmarkData) write(w *bufio.Writer) error {
	var stopRusage wsyscall.Rusage
	durationReal := time.Since(bd.startTime)
	err := wsyscall.Getrusage(wsyscall.RUSAGE_THREAD, &stopRusage)
	if err != nil {
		fmt.Fprintf(w,
			"<br><font color=\"grey\">Render time: real: %s  wbuf: %d B</font>\n",
			durationReal, w.Buffered())
		return err
	}
	runtime.UnlockOSThread()
	var durationUser, durationSys int64
	durationUser = (stopRusage.Utime.Sec - bd.startRusage.Utime.Sec) * 1000000
	durationUser += stopRusage.Utime.Usec - bd.startRusage.Utime.Usec
	durationSys = (stopRusage.Stime.Sec - bd.startRusage.Stime.Sec) * 1000000
	durationSys += stopRusage.Stime.Usec - bd.startRusage.Stime.Usec
	fmt.Fprintf(w,
		"<br><font color=\"grey\">Render time: real: %s, user: %d us, sys: %d us  wbuf: %d B</font>\n",
		durationReal, durationUser, durationSys, w.Buffered())
	return nil
}

func benchmarkedHandler(handler func(io.Writer,
	*http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		bd, _ := CreateBenchmarkData()
		writer := bufio.NewWriter(w)
		defer writer.Flush()
		defer fmt.Fprintln(writer, "</body>")
		handler(writer, req)
		bd.Write(writer)
	}
}
