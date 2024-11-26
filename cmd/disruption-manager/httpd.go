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
	dm_proto "github.com/Cloud-Foundations/Dominator/proto/disruptionmanager"
	sub_proto "github.com/Cloud-Foundations/Dominator/proto/sub"
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
	var request dm_proto.DisruptionRequest
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var err error
	var state sub_proto.DisruptionState
	var logMessage string
	switch request.Request {
	case sub_proto.DisruptionRequestCancel:
		err = hostAccessCheck(req.RemoteAddr, request.MDB.Hostname)
		if err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		state, logMessage, err = s.disruptionManager.cancel(request.MDB)
	case sub_proto.DisruptionRequestCheck:
		state, logMessage, err = s.disruptionManager.check(request.MDB)
	case sub_proto.DisruptionRequestRequest:
		err = hostAccessCheck(req.RemoteAddr, request.MDB.Hostname)
		if err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		state, logMessage, err = s.disruptionManager.request(request.MDB)
	default:
		err = fmt.Errorf("invalid request: %d", request.Request)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	} else {
		if logMessage != "" {
			s.logger.Println(logMessage)
		}
		writer := bufio.NewWriter(w)
		defer writer.Flush()
		reply := dm_proto.DisruptionResponse{Response: state}
		if err := libjson.WriteWithIndent(writer, "    ", reply); err != nil {
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
		"Hostname", "State", "Group", "Request Age/Timeout", "Ready Timeout",
		"Ready URL")
	groupList := s.disruptionManager.getGroupList()
	now := time.Now()
	for _, groupInfo := range groupList.groups {
		if len(groupInfo.Permitted) < 1 &&
			len(groupInfo.Requested) < 1 &&
			len(groupInfo.Waiting) < 1 {
			continue
		}
		for _, hostInfo := range groupInfo.Permitted {
			tw.WriteRow("", "",
				hostInfo.Hostname, "permitted", groupInfo.Identifier,
				format.Duration(now.Sub(hostInfo.LastRequest))+"/"+
					format.Duration(hostInfo.LastRequest.Add(
						s.disruptionManager.maxDuration).Sub(now)),
				"", "")
		}
		for _, hostInfo := range groupInfo.Requested {
			tw.WriteRow("", "",
				hostInfo.Hostname, "requested", groupInfo.Identifier,
				format.Duration(now.Sub(hostInfo.LastRequest))+"/"+
					format.Duration(hostInfo.LastRequest.Add(
						s.disruptionManager.maxDuration).Sub(now)),
				"", "")
		}
		for _, waitInfo := range groupInfo.Waiting {
			tw.WriteRow("", "",
				waitInfo.Hostname, "waiting", groupInfo.Identifier,
				"", format.Duration(waitInfo.ReadyTimeout.Sub(now)),
				waitInfo.ReadyUrl)
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
		fmt.Fprintln(writer,
			"No disruptions permitted, requested or waiting<br>")
	} else {
		fmt.Fprintf(writer,
			"%d disruptions permitted, %d requested and %d waiting: ",
			groupList.totalPermitted, groupList.totalRequested,
			groupList.totalWaiting)
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
