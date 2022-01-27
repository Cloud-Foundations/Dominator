package herd

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/html"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

type imageSubType struct {
	Hostname            string
	LastSuccessfulImage string `json:",omitempty"`
	PlannedImage        string `json:",omitempty"`
	RequiredImage       string `json:",omitempty"`
	Status              string
}

func (herd *Herd) listImagesForSubsHandler(w http.ResponseWriter,
	req *http.Request) {
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	herd.showImagesForSubsJSON(writer)
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
		"Planned Image", "Status", "Last Image Update")
	subs := herd.getSelectedSubs(selectAliveSub)
	for _, sub := range subs {
		showImagesForSub(tw, sub)
	}
	fmt.Fprintln(writer, "</table>")
}

func (herd *Herd) showImagesForSubsJSON(writer io.Writer) {
	subs := herd.getSelectedSubs(selectAliveSub)
	output := make([]imageSubType, 0, len(subs))
	for _, sub := range subs {
		output = append(output, imageSubType{
			Hostname:            sub.mdb.Hostname,
			PlannedImage:        sub.mdb.PlannedImage,
			LastSuccessfulImage: sub.lastSuccessfulImageName,
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
}
