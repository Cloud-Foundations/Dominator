package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/html"
	libjson "github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	proto "github.com/Cloud-Foundations/Dominator/proto/disruptionmanager"
)

type HtmlWriter interface {
	WriteHtml(writer io.Writer)
}

type httpServer struct {
	disruptionManager *disruptionManager
	htmlWriters       []HtmlWriter
	logger            log.DebugLogger
}

func startHttpServer(dm *disruptionManager,
	logger log.DebugLogger) (*httpServer, error) {
	s := &httpServer{
		disruptionManager: dm,
		logger:            logger,
	}
	html.HandleFunc("/", s.statusHandler)
	html.HandleFunc("/api/v1/request", s.requestHandler)
	html.HandleFunc("/showState", s.showStateHandler)
	return s, nil
}

func (s *httpServer) AddHtmlWriter(htmlWriter HtmlWriter) {
	s.htmlWriters = append(s.htmlWriters, htmlWriter)
}

func (s *httpServer) requestHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		http.Error(w, "unsupported method", http.StatusMethodNotAllowed)
		return
	}
	var request proto.DisruptionRequest
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if reply, err := s.disruptionManager.processRequest(request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	} else {
		writer := bufio.NewWriter(w)
		defer writer.Flush()
		if err := libjson.WriteWithIndent(writer, "    ", *reply); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (s *httpServer) serve(portNum uint) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", portNum))
	if err != nil {
		return err
	}
	return http.Serve(listener, nil)
}

func (s *httpServer) showStateHandler(w http.ResponseWriter,
	req *http.Request) {
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	fmt.Fprintln(writer, "<title>Disruption Manager disruptions page</title>")
	fmt.Fprintln(writer, `<style>
	                          table, th, td {
	                          border-collapse: collapse;
	                          }
	                          </style>`)
	fmt.Fprintln(writer, "<body>")
	fmt.Fprintln(writer, "<center>")
	fmt.Fprintln(writer, `<table border="1" style="width:100%">`)
	tw, _ := html.NewTableWriter(writer, true,
		"Hostname", "State", "Group", "Request Age")
	groupList := s.disruptionManager.getGroupList()
	now := time.Now()
	for _, groupInfo := range groupList.groups {
		if len(groupInfo.Permitted) < 1 && len(groupInfo.Requested) < 1 {
			continue
		}
		for _, hostInfo := range groupInfo.Permitted {
			tw.WriteRow("", "",
				hostInfo.Hostname, "permitted", groupInfo.Identifier,
				format.Duration(now.Sub(hostInfo.LastRequest)))
		}
		for _, hostInfo := range groupInfo.Requested {
			tw.WriteRow("", "",
				hostInfo.Hostname, "requested", groupInfo.Identifier,
				format.Duration(now.Sub(hostInfo.LastRequest)))
		}
	}
	tw.Close()
	fmt.Fprintln(writer, "</center>")
	fmt.Fprintln(writer, "</body>")
}

func (s *httpServer) statusHandler(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		http.NotFound(w, req)
		return
	}
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	fmt.Fprintln(writer, "<title>Disruption Manager status page</title>")
	fmt.Fprintln(writer, `<style>
	                          table, th, td {
	                          border-collapse: collapse;
	                          }
	                          </style>`)
	fmt.Fprintln(writer, "<body>")
	fmt.Fprintln(writer, "<center>")
	fmt.Fprintln(writer, "<h1><b>Disruption Manager</b> status page</h1>")
	fmt.Fprintln(writer, "</center>")
	html.WriteHeaderWithRequestNoGC(writer, req)
	fmt.Fprintln(writer, "<h3>")
	groupList := s.disruptionManager.getGroupList()
	if len(groupList.groups) < 1 {
		fmt.Fprintln(writer, "No disruptions permitted or requested<br>")
	} else {
		fmt.Fprintf(writer, "%d disruptions permitted and %d requested: ",
			groupList.totalPermitted, groupList.totalRequested)
		fmt.Fprintln(writer, `<a href="showState">dashboard</a><br>`)
	}
	for _, htmlWriter := range s.htmlWriters {
		htmlWriter.WriteHtml(writer)
	}
	fmt.Fprintln(writer, "</h3>")
	fmt.Fprintln(writer, "<hr>")
	html.WriteFooter(writer)
	fmt.Fprintln(writer, "</body>")
}
