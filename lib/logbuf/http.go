package logbuf

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/html"
	liburl "github.com/Cloud-Foundations/Dominator/lib/url"
	_ "github.com/Cloud-Foundations/tricorder/go/healthserver"
)

type countingWriter struct {
	count      uint64
	writer     io.Writer
	prefixLine string
}

// parseRegexp will parse "exclude=" and "include=" queries which must contain
// regular expressions. It will return a list of exclude and include compiled
// regular expressions or nil if there are none, and true on success. On failure
// (such as for a parse error), it writes an error message and returns
// nil, false.
func parseRegexp(w http.ResponseWriter, req *http.Request) (
	*regexpListType, bool) {
	regexpList := &regexpListType{}
	for _, value := range req.URL.Query()["exclude"] {
		expression, err := url.QueryUnescape(value)
		if err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, err.Error())
			return nil, false
		}
		re, err := regexp.Compile(expression)
		if err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, err.Error())
			return nil, false
		}
		regexpList.excludeList = append(regexpList.excludeList, re)
	}
	for _, value := range req.URL.Query()["include"] {
		expression, err := url.QueryUnescape(value)
		if err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, err.Error())
			return nil, false
		}
		re, err := regexp.Compile(expression)
		if err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, err.Error())
			return nil, false
		}
		regexpList.includeList = append(regexpList.includeList, re)
	}
	if len(regexpList.excludeList) < 1 && len(regexpList.includeList) < 1 {
		return nil, true
	}
	return regexpList, true
}

func showRecentLinks(w io.Writer, recentFirstString string) {
	var recentFirstStringOnly string
	if recentFirstString != "" && recentFirstString[0] == '&' {
		recentFirstStringOnly = "?" + recentFirstString[1:]
	}
	fmt.Fprintf(w, "Show last: <a href=\"logs/showLast?1m%s\">minute</a>\n",
		recentFirstString)
	fmt.Fprintf(w, "           <a href=\"logs/showLast?10m%s\">10 min</a>\n",
		recentFirstString)
	fmt.Fprintf(w, "           <a href=\"logs/showLast?1h%s\">hour</a>\n",
		recentFirstString)
	fmt.Fprintf(w, "           <a href=\"logs/showLast?1d%s\">day</a>\n",
		recentFirstString)
	fmt.Fprintf(w, "           <a href=\"logs/showLast?1w%s\">week</a>\n",
		recentFirstString)
	fmt.Fprintf(w, "           <a href=\"logs/showLast?1M%s\">month</a>\n",
		recentFirstString)
	fmt.Fprintf(w, "           <a href=\"logs/showLast?1y%s\">year</a>\n",
		recentFirstString)
	fmt.Fprintf(w, "           <a href=\"logs/showSinceStartup%s\">since startup</a><br>\n",
		recentFirstStringOnly)
	fmt.Fprintln(w, `Show <a href="logs/showStackTrace">stack trace</a>`)
}

func (w *countingWriter) Write(p []byte) (n int, err error) {
	if w.prefixLine != "" {
		w.writer.Write([]byte(w.prefixLine))
		w.prefixLine = ""
	}
	n, err = w.writer.Write(p)
	if n > 0 {
		w.count += uint64(n)
	}
	return
}

func (lb *LogBuffer) addHttpHandlers() {
	if lb.options.HttpServeMux == nil {
		return
	}
	html.ServeMuxHandleFunc(lb.options.HttpServeMux, "/logs",
		lb.httpListHandler)
	html.ServeMuxHandleFunc(lb.options.HttpServeMux, "/logs/dump",
		lb.httpDumpHandler)
	html.ServeMuxHandleFunc(lb.options.HttpServeMux, "/logs/showLast",
		lb.httpShowLastHandler)
	html.ServeMuxHandleFunc(lb.options.HttpServeMux, "/logs/showPreviousPanic",
		lb.httpShowPreviousPanicHandler)
	html.ServeMuxHandleFunc(lb.options.HttpServeMux, "/logs/showSinceStartup",
		lb.httpShowSinceStartupHandler)
	html.ServeMuxHandleFunc(lb.options.HttpServeMux, "/logs/showStackTrace",
		lb.httpShowStackTraceHandler)
}

func (lb *LogBuffer) dumpFile(writer io.Writer, filename string,
	recentFirst bool, regexpList *regexpListType) error {
	file, err := os.Open(path.Join(lb.options.Directory, filename))
	if err != nil {
		return err
	}
	defer file.Close()
	if recentFirst || regexpList != nil {
		scanner := bufio.NewScanner(file)
		lines := make([]string, 0)
		for scanner.Scan() {
			line := scanner.Text()
			if len(line) < 1 {
				continue
			}
			if !regexpList.includeString(line) {
				continue
			}
			if !recentFirst {
				fmt.Fprintln(writer, line)
				continue
			}
			lines = append(lines, line)
		}
		if err = scanner.Err(); err == nil {
			reverseStrings(lines)
			for _, line := range lines {
				fmt.Fprintln(writer, line)
			}
		}
	} else {
		_, err = io.Copy(writer, bufio.NewReader(file))
	}
	return err
}

func (lb *LogBuffer) httpDumpHandler(w http.ResponseWriter, req *http.Request) {
	regexpList, ok := parseRegexp(w, req)
	if !ok {
		return
	}
	parsedQuery := liburl.ParseQuery(req.URL)
	name, ok := parsedQuery.Table["name"]
	if !ok {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	_, recentFirst := parsedQuery.Flags["recentFirst"]
	if name == "latest" {
		lbFilename := ""
		lb.rwMutex.RLock()
		if lb.file != nil {
			lbFilename = lb.file.Name()
		}
		lb.rwMutex.RUnlock()
		if lbFilename == "" {
			writer := bufio.NewWriter(w)
			defer writer.Flush()
			lb.dump(writer, "", "", recentFirst, false, regexpList)
			return
		}
		name = path.Base(lbFilename)
	}
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	err := lb.dumpFile(writer, path.Base(path.Clean(name)), recentFirst,
		regexpList)
	if err != nil && os.IsNotExist(err) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err != nil {
		fmt.Fprintln(writer, err)
	}
}

func (lb *LogBuffer) httpListHandler(w http.ResponseWriter, req *http.Request) {
	if lb.options.Directory == "" {
		return
	}
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	parsedQuery := liburl.ParseQuery(req.URL)
	_, recentFirst := parsedQuery.Flags["recentFirst"]
	names, panicMap, err := lb.list(recentFirst)
	if err != nil {
		fmt.Fprintln(writer, err)
		return
	}
	recentFirstString := ""
	if recentFirst {
		recentFirstString = "&recentFirst"
	}
	if parsedQuery.OutputType() == liburl.OutputTypeText {
		for _, name := range names {
			fmt.Fprintln(writer, name)
		}
		return
	}
	fmt.Fprintln(writer, "<body>")
	fmt.Fprint(writer, "Logs: ")
	if recentFirst {
		fmt.Fprintf(writer, "showing recent first ")
		fmt.Fprintln(writer, `<a href="logs">show recent last</a><br>`)
	} else {
		fmt.Fprintf(writer, "showing recent last ")
		fmt.Fprintln(writer,
			`<a href="logs?recentFirst">show recent first</a><br>`)
	}
	showRecentLinks(writer, recentFirstString)
	fmt.Fprintln(writer, "<p>")
	currentName := ""
	lb.rwMutex.RLock()
	if lb.file != nil {
		currentName = path.Base(lb.file.Name())
	}
	lb.rwMutex.RUnlock()
	if recentFirst {
		fmt.Fprintf(writer,
			"<a href=\"logs/dump?name=latest%s\">current</a><br>\n",
			recentFirstString)
	}
	for _, name := range names {
		if !recentFirst {
			if name == lb.firstFile {
				fmt.Fprintln(writer, "<hr>")
			}
		}
		if name == currentName {
			fmt.Fprintf(writer,
				"<a href=\"logs/dump?name=%s%s\">%s</a> (current)<br>\n",
				name, recentFirstString, name)
		} else {
			hasPanic := ""
			if _, ok := panicMap[name]; ok {
				hasPanic = " (has panic log)"
			}
			fmt.Fprintf(writer,
				"<a href=\"logs/dump?name=%s%s\">%s</a>%s<br>\n",
				name, recentFirstString, name, hasPanic)
		}
		if recentFirst {
			if name == lb.firstFile {
				fmt.Fprintln(writer, "<hr>")
			}
		}
	}
	if !recentFirst {
		fmt.Fprintf(writer,
			"<a href=\"logs/dump?name=latest%s\">current</a><br>\n",
			recentFirstString)
	}
	fmt.Fprintln(writer, "</body>")
}

func (lb *LogBuffer) httpShowLastHandler(w http.ResponseWriter,
	req *http.Request) {
	parsedQuery := liburl.ParseQuery(req.URL)
	_, recentFirst := parsedQuery.Flags["recentFirst"]
	regexpList, ok := parseRegexp(w, req)
	if !ok {
		return
	}
	for flag := range parsedQuery.Flags {
		length := len(flag)
		if length < 2 {
			continue
		}
		unitChar := flag[length-1]
		var unit time.Duration
		switch unitChar {
		case 's':
			unit = time.Second
		case 'm':
			unit = time.Minute
		case 'h':
			unit = time.Hour
		case 'd':
			unit = time.Hour * 24
		case 'w':
			unit = time.Hour * 24 * 7
		case 'M':
			// TODO(rgooch): Fix this crude approximation.
			unit = time.Hour * 24 * 31
		case 'y':
			unit = time.Hour * 24 * 365
		default:
			continue
		}
		if val, err := strconv.ParseUint(flag[:length-1], 10, 64); err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			return
		} else {
			lb.showRecent(w, time.Duration(val)*unit, recentFirst, regexpList)
			return
		}
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
}

func (lb *LogBuffer) httpShowPreviousPanicHandler(w http.ResponseWriter,
	req *http.Request) {
	regexpList, ok := parseRegexp(w, req)
	if !ok {
		return
	}
	parsedQuery := liburl.ParseQuery(req.URL)
	_, recentFirst := parsedQuery.Flags["recentFirst"]
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	panicLogfile := lb.panicLogfile
	if panicLogfile == nil {
		fmt.Fprintln(writer, "Last invocation did not panic!")
		return
	}
	if *panicLogfile == "" {
		fmt.Fprintln(writer, "Logfile for previous invocation has expired")
		return
	}
	err := lb.dumpFile(writer, *panicLogfile, recentFirst, regexpList)
	if err == nil {
		return
	}
	if os.IsNotExist(err) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	fmt.Fprintln(writer, err)
}

func (lb *LogBuffer) httpShowSinceStartupHandler(w http.ResponseWriter,
	req *http.Request) {
	regexpList, ok := parseRegexp(w, req)
	if !ok {
		return
	}
	parsedQuery := liburl.ParseQuery(req.URL)
	_, recentFirst := parsedQuery.Flags["recentFirst"]
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	names, _, err := lb.list(recentFirst)
	if err != nil {
		fmt.Fprintln(writer, err)
		return
	}
	lb.flush()
	dumpFiles := recentFirst
	for _, name := range names {
		if !recentFirst && name == lb.firstFile {
			dumpFiles = true
		}
		if dumpFiles {
			lb.dumpFile(writer, name, recentFirst, regexpList)
		}
		if recentFirst && name == lb.firstFile {
			dumpFiles = false
		}
	}
}

func (lb *LogBuffer) httpShowStackTraceHandler(w http.ResponseWriter,
	req *http.Request) {
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	lb.rwMutex.Lock()
	if time.Since(lb.lastStackTrace) < 2*time.Second {
		lb.rwMutex.Unlock()
		writer.Write([]byte("Too soon\n"))
		return
	}
	lb.lastStackTrace = time.Now()
	lb.rwMutex.Unlock()
	buffer := make([]byte, 1<<20)
	nBytes := runtime.Stack(buffer, true)
	writer.Write(buffer[:nBytes])
}

func (lb *LogBuffer) list(recentFirst bool) (
	[]string, map[string]struct{}, error) {
	file, err := os.Open(lb.options.Directory)
	if err != nil {
		return nil, nil, err
	}
	fileInfos, err := file.Readdir(-1)
	file.Close()
	if err != nil {
		return nil, nil, err
	}
	panicMap := make(map[string]struct{})
	names := make([]string, 0, len(fileInfos))
	for _, fi := range fileInfos {
		if strings.Count(fi.Name(), ":") == 3 {
			names = append(names, fi.Name())
			if fi.Mode()&os.ModeSticky != 0 {
				panicMap[fi.Name()] = struct{}{}
			}
		}
	}
	sort.Strings(names)
	if recentFirst {
		reverseStrings(names)
	}
	return names, panicMap, nil
}

func (lb *LogBuffer) showRecent(w io.Writer, duration time.Duration,
	recentFirst bool, excludeList *regexpListType) {
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	names, _, err := lb.list(true)
	if err != nil {
		fmt.Fprintln(writer, err)
		return
	}
	earliestTime := time.Now().Add(-duration)
	// Get a list of names which may be recent enough.
	tmpNames := make([]string, 0, len(names))
	for _, name := range names {
		startTime, err := time.ParseInLocation(timeLayout, name, time.Local)
		if err != nil {
			continue
		}
		tmpNames = append(tmpNames, name)
		if startTime.Before(earliestTime) {
			break
		}
	}
	names = tmpNames
	if !recentFirst {
		reverseStrings(names)
	}
	fmt.Fprintln(writer, "<body>")
	cWriter := &countingWriter{writer: writer, prefixLine: "<pre>\n"}
	lb.flush()
	for _, name := range names {
		cWriter.count = 0
		foundReopenMessage, _ := lb.dumpSince(cWriter, name, earliestTime, "",
			"\n", recentFirst, excludeList)
		if cWriter.count > 0 && !foundReopenMessage {
			cWriter.prefixLine = "</pre>\n<hr>\n<pre>\n"
		}
	}
	fmt.Fprintln(writer, "</pre>")
	fmt.Fprintln(writer, "</body>")
}

func (lb *LogBuffer) writeHtml(writer io.Writer) {
	if lb.options.Directory != "" {
		fmt.Fprintln(writer, `<a href="logs">Logs:</a><br>`)
		panicLogfile := lb.panicLogfile
		if panicLogfile != nil {
			fmt.Fprint(writer,
				"<font color=\"red\">Last invocation paniced</font>, ")
			if *panicLogfile == "" {
				fmt.Fprintln(writer, "logfile no longer available<br>")
			} else {
				fmt.Fprintln(writer,
					"<a href=\"logs/showPreviousPanic\">logfile</a><br>")
			}
		}
	} else {
		fmt.Fprintln(writer, "Logs:<br>")
	}
	fmt.Fprintln(writer, "<pre>")
	lb.dump(writer, "", "", false, true, nil)
	fmt.Fprintln(writer, "</pre>")
}
