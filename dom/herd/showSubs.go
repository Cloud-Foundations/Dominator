package herd

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/html"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
	"github.com/Cloud-Foundations/Dominator/lib/url"
	proto "github.com/Cloud-Foundations/Dominator/proto/dominator"
)

func makeSelector(locationsToMatch []string, statusesToMatch []string,
	tagsToMatch map[string][]string) func(sub *Sub) bool {
	if len(locationsToMatch) < 1 &&
		len(statusesToMatch) < 1 &&
		len(tagsToMatch) < 1 {
		return selectAll
	}
	locationsToMatchMap := stringutil.ConvertListToMap(locationsToMatch, false)
	statusesToMatchMap := stringutil.ConvertListToMap(statusesToMatch, false)
	return func(sub *Sub) bool {
		if len(locationsToMatch) > 0 {
			subLocationLength := len(sub.mdb.Location)
			if subLocationLength < 1 {
				return false
			}
			_, matched := locationsToMatchMap[sub.mdb.Location]
			if !matched {
				for _, locationToMatch := range locationsToMatch {
					index := len(locationToMatch)
					if index < subLocationLength &&
						sub.mdb.Location[index] == '/' &&
						strings.HasPrefix(sub.mdb.Location, locationToMatch) {
						matched = true
						break
					}
				}
			}
			if !matched {
				return false
			}
		}
		if len(statusesToMatch) > 0 {
			if _, ok := statusesToMatchMap[sub.status.String()]; !ok {
				return false
			}
		}
		for key, values := range tagsToMatch {
			var matchedTag bool
			for _, value := range values {
				if value == sub.mdb.Tags[key] {
					matchedTag = true
					break
				}
			}
			if !matchedTag {
				return false
			}
		}
		return true
	}
}

func makeUrlQuerySelector(queryValues map[string][]string) func(sub *Sub) bool {
	if len(queryValues) < 1 {
		return selectAll
	}
	tagsToMatch := make(map[string][]string)
	for _, queryTag := range queryValues["tag"] {
		split := strings.Split(queryTag, "=")
		if len(split) != 2 {
			continue
		}
		key := split[0]
		value := split[1]
		tagsToMatch[key] = append(tagsToMatch[key], value)
	}
	return makeSelector(queryValues["location"], queryValues["status"],
		tagsToMatch)
}

func selectAll(sub *Sub) bool {
	return true
}

func (herd *Herd) makeShowSubsHandler(selectFunc func(*Sub) bool,
	subType string) func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, req *http.Request) {
		herd.showSubsHandler(writer, req, selectFunc, subType)
	}
}

func (herd *Herd) showReachableSubsHandler(writer http.ResponseWriter,
	req *http.Request) {
	selectFunc, _ := herd.getReachableSelector(url.ParseQuery(req.URL))
	herd.showSubsHandler(writer, req, selectFunc, "reachable ")
}

func (herd *Herd) showSubsCSV(writer io.Writer,
	selectFunc func(*Sub) bool) {
	subs := herd.getSelectedSubs(selectFunc)
	w := csv.NewWriter(writer)
	defer w.Flush()
	w.Write([]string{
		"Hostname",
		"Required Image",
		"Planned Image",
		"Status",
		"Last Image Update",
		"Last Note",
	})
	for _, sub := range subs {
		w.Write([]string{
			sub.mdb.Hostname,
			sub.mdb.RequiredImage,
			sub.mdb.PlannedImage,
			sub.publishedStatus.String(),
			sub.lastSuccessfulImageName,
			sub.lastNote,
		})
	}
}

func (herd *Herd) showSubsHandler(rWriter http.ResponseWriter,
	req *http.Request, _selectFunc func(*Sub) bool,
	subType string) {
	querySelectFunc := makeUrlQuerySelector(req.URL.Query())
	selectFunc := func(sub *Sub) bool {
		return _selectFunc(sub) && querySelectFunc(sub)
	}
	writer := bufio.NewWriter(rWriter)
	defer writer.Flush()
	parsedQuery := url.ParseQuery(req.URL)
	switch parsedQuery.OutputType() {
	case url.OutputTypeCsv:
		herd.showSubsCSV(writer, selectFunc)
	case url.OutputTypeHtml:
		herd.showSubsHTML(writer, selectFunc, subType)
	case url.OutputTypeJson:
		herd.showSubsJSON(writer, selectFunc)
	case url.OutputTypeText:
		fmt.Fprintln(writer, "Text output not supported")
	default:
		fmt.Fprintln(writer, "Unknown output type")
	}
}

func (herd *Herd) showSubsHTML(writer *bufio.Writer, selectFunc func(*Sub) bool,
	subType string) {
	bd, _ := html.CreateBenchmarkData()
	defer fmt.Fprintln(writer, "</body>")
	fmt.Fprintf(writer, "<title>Dominator %s subs</title>", subType)
	fmt.Fprintln(writer, `<style>
                          table, th, td {
                          border-collapse: collapse;
                          }
                          </style>`)
	if srpc.CheckTlsRequired() {
		fmt.Fprintln(writer, "<body>")
	} else {
		fmt.Fprintln(writer, "<body bgcolor=\"#ffb0b0\">")
		fmt.Fprintln(writer,
			`<h1><center><font color="red">Running in insecure mode. You can get pwned!!!</center></font></h1>`)
	}
	if herd.updatesDisabledReason != "" {
		fmt.Fprintf(writer, "<center>")
		herd.writeDisableStatus(writer)
		fmt.Fprintln(writer, "</center>")
	}
	fmt.Fprintln(writer, `<table border="1" style="width:100%">`)
	tw, _ := html.NewTableWriter(writer, true, "Name", "Required Image",
		"Planned Image", "Busy", "Status", "Uptime", "Last Scan Duration",
		"Staleness", "Last Update", "Last Sync", "Connect", "Short Poll",
		"Full Poll", "Update Compute")
	subs := herd.getSelectedSubs(selectFunc)
	for _, sub := range subs {
		showSub(tw, sub)
	}
	fmt.Fprintln(writer, "</table>")
	bd.Write(writer)
}

func (herd *Herd) showSubsJSON(writer io.Writer,
	selectFunc func(*Sub) bool) {
	subs := herd.getSelectedSubs(selectFunc)
	output := make([]proto.SubInfo, 0, len(subs))
	for _, sub := range subs {
		output = append(output, sub.makeInfo())
	}
	json.WriteWithIndent(writer, "   ", output)
}

func (sub *Sub) makeInfo() proto.SubInfo {
	return proto.SubInfo{
		Machine:             sub.mdb,
		LastDisruptionState: sub.lastDisruptionState,
		LastNote:            sub.lastNote,
		LastScanDuration:    sub.lastScanDuration,
		LastSuccessfulImage: sub.lastSuccessfulImageName,
		LastSyncTime:        sub.lastSyncTime,
		LastUpdateTime:      sub.lastUpdateTime,
		StartTime:           sub.startTime,
		Status:              sub.publishedStatus.String(),
		SystemUptime:        sub.systemUptime,
	}
}

func showSub(tw *html.TableWriter, sub *Sub) {
	var background string
	if sub.isInsecure {
		background = "yellow"
	}
	tw.OpenRow("", background)
	defer tw.CloseRow()
	subURL := fmt.Sprintf("http://%s:%d/",
		strings.SplitN(sub.String(), "*", 2)[0], constants.SubPortNumber)
	timeNow := time.Now()
	tw.WriteData("", fmt.Sprintf("<a href=\"%s\">%s</a>", subURL, sub))
	sub.herd.showImage(tw, sub.mdb.RequiredImage, true)
	sub.herd.showImage(tw, sub.mdb.PlannedImage, false)
	sub.showBusy(tw)
	tw.WriteData("",
		fmt.Sprintf("<a href=\"showSub?%s\">%s</a>",
			sub.mdb.Hostname, sub.publishedStatus.html()))
	showSince(tw, sub.pollTime, sub.startTime)
	showDuration(tw, sub.lastScanDuration, false)
	showSince(tw, timeNow, sub.lastPollSucceededTime)
	showSince(tw, timeNow, sub.lastUpdateTime)
	showSince(tw, timeNow, sub.lastSyncTime)
	showDuration(tw, sub.lastConnectDuration, false)
	showDuration(tw, sub.lastShortPollDuration, !sub.lastPollWasFull)
	showDuration(tw, sub.lastFullPollDuration, sub.lastPollWasFull)
	showDuration(tw, sub.lastComputeUpdateCpuDuration, false)
}

func (herd *Herd) showImage(tw *html.TableWriter, name string,
	showDefault bool) error {
	if name == "" {
		if showDefault && herd.defaultImageName != "" {
			return tw.WriteData("", fmt.Sprintf(
				"<a style=\"color: #CCCC00\" href=\"http://%s/showImage?%s\">%s</a>",
				herd.imageManager, herd.defaultImageName,
				herd.defaultImageName))
		} else {
			return tw.WriteData("", "")
		}
	} else if image, err := herd.imageManager.Get(name, false); err != nil {
		return tw.WriteData("red", err.Error())
	} else if image != nil {
		return tw.WriteData("",
			fmt.Sprintf("<a href=\"http://%s/showImage?%s\">%s</a>",
				herd.imageManager, name, name))
	} else {
		return tw.WriteData("grey", name)
	}
}

func (herd *Herd) showSubHandler(writer http.ResponseWriter,
	req *http.Request) {
	w := bufio.NewWriter(writer)
	defer w.Flush()
	subName := strings.Split(req.URL.RawQuery, "&")[0]
	parsedQuery := url.ParseQuery(req.URL)
	sub := herd.getSub(subName)
	if sub == nil {
		http.NotFound(writer, req)
		return
	}
	if parsedQuery.OutputType() == url.OutputTypeJson {
		json.WriteWithIndent(w, "    ", sub.makeInfo())
		return
	}
	fmt.Fprintf(w, "<title>sub %s</title>", subName)
	if srpc.CheckTlsRequired() {
		fmt.Fprintln(w, "<body>")
	} else {
		fmt.Fprintln(w, "<body bgcolor=\"#ffb0b0\">")
		fmt.Fprintln(w,
			`<h1><center><font color="red">Running in insecure mode. You can get pwned!!!</center></font></h1>`)
	}
	if herd.updatesDisabledReason != "" {
		fmt.Fprintf(w, "<center>")
		herd.writeDisableStatus(w)
		fmt.Fprintln(w, "</center>")
	}
	fmt.Fprintln(w, "<h3>")
	timeNow := time.Now()
	subURL := fmt.Sprintf("http://%s:%d/",
		strings.SplitN(sub.String(), "*", 2)[0], constants.SubPortNumber)
	fmt.Fprintf(w,
		"Information for sub: <a href=\"%s\">%s</a> (<a href=\"%s?%s&output=json\">JSON</a>)<br>\n",
		subURL, subName, req.URL.Path, subName)
	fmt.Fprintln(w, "</h3>")
	fmt.Fprint(w, "<table border=\"0\">\n")
	tw, _ := html.NewTableWriter(w, false)
	newRow(w, "Required Image", true)
	sub.herd.showImage(tw, sub.mdb.RequiredImage, true)
	newRow(w, "Planned Image", false)
	sub.herd.showImage(tw, sub.mdb.PlannedImage, false)
	newRow(w, "Last successful image update", false)
	sub.herd.showImage(tw, sub.lastSuccessfulImageName, false)
	if sub.lastNote != "" {
		newRow(w, "Last note", false)
		tw.WriteData("", sub.lastNote)
	}
	newRow(w, "Busy time", false)
	sub.showBusy(tw)
	newRow(w, "Status", false)
	tw.WriteData("", sub.publishedStatus.html())
	if sub.lastWriteError != "" {
		newRow(w, "Last write error", false)
		tw.WriteData("", sub.lastWriteError)
	}
	newRow(w, "Uptime", false)
	showSince(tw, sub.pollTime, sub.startTime)
	newRow(w, "Last scan duration", false)
	showDuration(tw, sub.lastScanDuration, false)
	if sub.mdb.Location != "" {
		newRow(w, "Location", false)
		tw.WriteData("", sub.mdb.Location)
	}
	newRow(w, "Time since last successful poll", false)
	showSince(tw, timeNow, sub.lastPollSucceededTime)
	newRow(w, "Time since last update", false)
	showSince(tw, timeNow, sub.lastUpdateTime)
	newRow(w, "Time since last sync", false)
	showSince(tw, timeNow, sub.lastSyncTime)
	newRow(w, "Last connection duration", false)
	showDuration(tw, sub.lastConnectDuration, false)
	newRow(w, "Last short poll duration", false)
	showDuration(tw, sub.lastShortPollDuration, !sub.lastPollWasFull)
	newRow(w, "Last full poll duration", false)
	showDuration(tw, sub.lastFullPollDuration, sub.lastPollWasFull)
	newRow(w, "Last compute duration", false)
	showDuration(tw, sub.lastComputeUpdateCpuDuration, false)
	newRow(w, "Last disruption state", false)
	tw.WriteData("", sub.lastDisruptionState.String())
	if sub.systemUptime != nil {
		newRow(w, "System uptime", false)
		showDuration(tw, *sub.systemUptime, false)
	}
	fmt.Fprint(w, "  </tr>\n")
	fmt.Fprint(w, "</table>\n")
	fmt.Fprintln(w, "MDB Data:")
	fmt.Fprintln(w, "<pre>")
	json.WriteWithIndent(w, "    ", sub.mdb)
	fmt.Fprintln(w, "</pre>")
}

func newRow(w io.Writer, row string, first bool) {
	if !first {
		fmt.Fprint(w, "  </tr>\n")
	}
	fmt.Fprint(w, "  <tr>\n")
	fmt.Fprintf(w, "    <td>%s:</td>\n", row)
}

func (sub *Sub) showBusy(tw *html.TableWriter) {
	if sub.busy {
		if sub.busyStartTime.IsZero() {
			tw.WriteData("", "busy")
		} else {
			tw.WriteData("", format.Duration(time.Since(sub.busyStartTime)))
		}
	} else {
		if sub.busyStartTime.IsZero() {
			tw.WriteData("", "")
		} else {
			tw.WriteData("grey",
				format.Duration(sub.busyStopTime.Sub(sub.busyStartTime)))
		}
	}
}

func showSince(tw *html.TableWriter, now time.Time, since time.Time) {
	if now.IsZero() || since.IsZero() {
		tw.WriteData("", "")
	} else {
		showDuration(tw, now.Sub(since), false)
	}
}

func showDuration(tw *html.TableWriter, duration time.Duration,
	highlight bool) {
	if duration < 1 {
		tw.WriteData("", "")
	} else {
		str := format.Duration(duration)
		if highlight {
			str = "<b>" + str + "</b>"
		}
		tw.WriteData("", str)
	}
}
