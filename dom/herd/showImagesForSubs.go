package herd

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/html"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/tags/tagmatcher"
	"github.com/Cloud-Foundations/Dominator/lib/url"
	proto "github.com/Cloud-Foundations/Dominator/proto/dominator"
)

func (herd *Herd) getInfoForSubs(request proto.GetInfoForSubsRequest) (
	[]proto.SubInfo, error) {
	selectFunc := makeSelector(request.LocationsToMatch,
		request.StatusesToMatch, tagmatcher.New(request.TagsToMatch, false))
	if len(request.Hostnames) < 1 {
		herd.RLock()
		defer herd.RUnlock()
		subInfos := make([]proto.SubInfo, 0, len(herd.subsByIndex))
		for _, sub := range herd.subsByIndex {
			if selectFunc(sub) {
				subInfos = append(subInfos, sub.makeInfo())
			}
		}
		return subInfos, nil
	}
	subInfos := make([]proto.SubInfo, 0, len(request.Hostnames))
	herd.RLock()
	defer herd.RUnlock()
	for _, hostname := range request.Hostnames {
		if sub, ok := herd.subsByName[hostname]; ok {
			if selectFunc(sub) {
				subInfos = append(subInfos, sub.makeInfo())
			}
		}
	}
	return subInfos, nil
}

func (herd *Herd) listImagesForSubsHandler(w http.ResponseWriter,
	req *http.Request) {
	querySelectFunc := makeUrlQuerySelector(req.URL.Query())
	selectFunc := func(sub *Sub) bool {
		return selectAliveSub(sub) && querySelectFunc(sub)
	}
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	parsedQuery := url.ParseQuery(req.URL)
	switch parsedQuery.OutputType() {
	case url.OutputTypeCsv:
		herd.showSubsCSV(writer, selectFunc)
	case url.OutputTypeHtml: // Want the benchmarking endpoint instead.
		fmt.Fprintln(writer, "HTML output not supported")
	case url.OutputTypeJson:
		herd.showSubsJSON(writer, selectFunc)
	case url.OutputTypeText:
		fmt.Fprintln(writer, "Text output not supported")
	default:
		fmt.Fprintln(writer, "Unknown output type")
	}
}

func (herd *Herd) showImagesForSubsHandler(w io.Writer, req *http.Request) {
	querySelectFunc := makeUrlQuerySelector(req.URL.Query())
	herd.showImagesForSubsHTML(w, func(sub *Sub) bool {
		return selectAliveSub(sub) && querySelectFunc(sub)
	})
}

func (herd *Herd) showImagesForSubsHTML(writer io.Writer,
	selectFunc func(*Sub) bool) {
	fmt.Fprintf(writer, "<title>Dominator images for subs</title>")
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
		"Planned Image", "Status", "Last Image Update", "Last Note")
	subs := herd.getSelectedSubs(selectFunc)
	for _, sub := range subs {
		showImagesForSub(tw, sub)
	}
	tw.Close()
}

func showImagesForSub(tw *html.TableWriter, sub *Sub) {
	var background string
	if sub.isInsecure {
		background = "yellow"
	}
	tw.OpenRow("", background)
	defer tw.CloseRow()
	subURL := fmt.Sprintf("http://%s:%d/",
		strings.SplitN(sub.String(), "*", 2)[0], constants.SubPortNumber)
	tw.WriteData("", fmt.Sprintf("<a href=\"%s\">%s</a>", subURL, sub))
	sub.herd.showImage(tw, sub.mdb.RequiredImage, true)
	sub.herd.showImage(tw, sub.mdb.PlannedImage, false)
	tw.WriteData("",
		fmt.Sprintf("<a href=\"showSub?%s\">%s</a>",
			sub.mdb.Hostname, sub.publishedStatus.html()))
	sub.herd.showImage(tw, sub.lastSuccessfulImageName, false)
	tw.WriteData("", sub.lastNote)
}
