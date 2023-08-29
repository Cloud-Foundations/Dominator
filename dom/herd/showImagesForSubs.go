package herd

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/html"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/url"
)

type imageSubType struct {
	Hostname            string
	LastNote            string `json:",omitempty"`
	LastSuccessfulImage string `json:",omitempty"`
	PlannedImage        string `json:",omitempty"`
	RequiredImage       string `json:",omitempty"`
	Status              string
}

func (herd *Herd) listImagesForSubsHandler(w http.ResponseWriter,
	req *http.Request) {
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	parsedQuery := url.ParseQuery(req.URL)
	switch parsedQuery.OutputType() {
	case url.OutputTypeCsv:
		herd.showImagesForSubsCSV(writer)
	case url.OutputTypeHtml: // Want the benchmarking endpoint instead.
		fmt.Fprintln(writer, "HTML output not supported")
	case url.OutputTypeJson:
		herd.showImagesForSubsJSON(writer)
	case url.OutputTypeText:
		fmt.Fprintln(writer, "Text output not supported")
	}
	fmt.Fprintln(writer, "Unknown output type")
}

func (herd *Herd) showImagesForSubsHandler(w io.Writer, req *http.Request) {
	herd.showImagesForSubsHTML(w)
}

func (herd *Herd) showImagesForSubsHTML(writer io.Writer) {
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
	subs := herd.getSelectedSubs(selectAliveSub)
	for _, sub := range subs {
		showImagesForSub(tw, sub)
	}
	fmt.Fprintln(writer, "</table>")
}

func (herd *Herd) showImagesForSubsCSV(writer io.Writer) {
	subs := herd.getSelectedSubs(selectAliveSub)
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

func (herd *Herd) showImagesForSubsJSON(writer io.Writer) {
	subs := herd.getSelectedSubs(selectAliveSub)
	output := make([]imageSubType, 0, len(subs))
	for _, sub := range subs {
		output = append(output, imageSubType{
			Hostname:            sub.mdb.Hostname,
			LastNote:            sub.lastNote,
			LastSuccessfulImage: sub.lastSuccessfulImageName,
			PlannedImage:        sub.mdb.PlannedImage,
			RequiredImage:       sub.mdb.RequiredImage,
			Status:              sub.publishedStatus.String(),
		})
	}
	json.WriteWithIndent(writer, "   ", output)
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
