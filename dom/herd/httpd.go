package herd

import (
	"fmt"
	"net"
	"net/http"

	"github.com/Cloud-Foundations/Dominator/lib/html"
)

func (herd *Herd) startServer(portNum uint, daemon bool) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", portNum))
	if err != nil {
		return err
	}
	html.HandleFunc("/", herd.statusHandler)
	html.HandleFunc("/listImagesForSubs", herd.listImagesForSubsHandler)
	html.HandleFunc("/listReachableSubs", herd.listReachableSubsHandler)
	html.HandleFunc("/listUnreachableSubs", herd.listUnreachableSubsHandler)
	html.HandleFunc("/listSubs", herd.listSubsHandler)
	html.HandleFunc("/showAliveSubs",
		herd.makeShowSubsHandler(selectAliveSub, "alive"))
	html.HandleFunc("/showAllSubs",
		herd.makeShowSubsHandler(selectAll, ""))
	html.HandleFunc("/showCompliantSubs",
		herd.makeShowSubsHandler(selectCompliantSub, "compliant "))
	html.HandleFunc("/showLikelyCompliantSubs",
		herd.makeShowSubsHandler(selectLikelyCompliantSub, "likely compliant "))
	html.HandleFunc("/showDeviantSubs",
		herd.makeShowSubsHandler(selectDeviantSub, "deviant "))
	html.HandleFunc("/showImagesForSubs",
		html.BenchmarkedHandler(herd.showImagesForSubsHandler))
	html.HandleFunc("/showReachableSubs", herd.showReachableSubsHandler)
	html.HandleFunc("/showUnreachableSubs", herd.showUnreachableSubsHandler)
	html.HandleFunc("/showSub", herd.showSubHandler)
	if daemon {
		go http.Serve(listener, nil)
	} else {
		http.Serve(listener, nil)
	}
	return nil
}

func (herd *Herd) addHtmlWriter(htmlWriter HtmlWriter) {
	herd.htmlWriters = append(herd.htmlWriters, htmlWriter)
}
