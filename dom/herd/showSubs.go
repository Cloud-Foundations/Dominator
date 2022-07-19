package herd

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	net_url "net/url"
	"strings"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/html"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/url"
	proto "github.com/Cloud-Foundations/Dominator/proto/dominator"
)

type subInfoType struct {
	Info proto.SubInfo
	MDB  mdb.Machine
}

func (herd *Herd) showAliveSubsHandler(w io.Writer, req *http.Request) {
	herd.showSubs(w, "alive ", selectAliveSub)
}

func (herd *Herd) showAllSubsHandler(w io.Writer, req *http.Request) {
	parsedQuery := url.ParseQuery(req.URL)
	var statusToMatch string
	if uesc, e := net_url.QueryUnescape(parsedQuery.Table["status"]); e == nil {
		statusToMatch = uesc
	}
	var selectFunc func(*Sub) bool
	if statusToMatch != "" {
		selectFunc = func(sub *Sub) bool {
			if sub.status.String() == statusToMatch {
				return true
			}
			return false
		}
	}
	herd.showSubs(w, "", selectFunc)
}

func (herd *Herd) showCompliantSubsHandler(w io.Writer, req *http.Request) {
	herd.showSubs(w, "compliant ", selectCompliantSub)
}

func (herd *Herd) showLikelyCompliantSubsHandler(w io.Writer,
	req *http.Request) {
	herd.showSubs(w, "likely compliant ", selectLikelyCompliantSub)
}

func (herd *Herd) showDeviantSubsHandler(w io.Writer, req *http.Request) {
	herd.showSubs(w, "deviant ", selectDeviantSub)
}

func (herd *Herd) showReachableSubsHandler(w io.Writer, req *http.Request) {
	selector, err := herd.getReachableSelector(url.ParseQuery(req.URL))
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}
	herd.showSubs(w, "reachable ", selector)
}

func (herd *Herd) showSubs(writer io.Writer, subType string,
	selectFunc func(*Sub) bool) {
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
		subInfo := subInfoType{
			Info: sub.makeInfo(),
			MDB:  sub.mdb,
		}
		json.WriteWithIndent(w, "    ", subInfo)
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
	newRow(w, "Uptime", false)
	showSince(tw, sub.pollTime, sub.startTime)
	newRow(w, "Last scan duration", false)
	showDuration(tw, sub.lastScanDuration, false)
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
