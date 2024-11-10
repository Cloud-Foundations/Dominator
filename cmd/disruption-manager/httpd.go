package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"

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
	groupList := s.disruptionManager.getGroupList()
	fmt.Fprintln(writer, "<pre>")
	for _, groupInfo := range groupList.groups {
		if len(groupInfo.Permitted) < 1 && len(groupInfo.Requested) < 1 {
			continue
		}
		if groupInfo.Identifier == "" {
			fmt.Fprintln(writer, "Global disruption group:")
		} else {
			fmt.Fprintf(writer, "Disruption group \"%s\":\n",
				groupInfo.Identifier)
		}
		if len(groupInfo.Permitted) > 0 {
			fmt.Fprintln(writer, "  Hosts permitted to disrupt:")
			for _, hostname := range groupInfo.Permitted {
				fmt.Fprintf(writer, "    %s\n", hostname)
			}
		}
		if len(groupInfo.Requested) > 0 {
			fmt.Fprintln(writer, "  Hosts requesting to disrupt:")
			for _, hostname := range groupInfo.Requested {
				fmt.Fprintf(writer, "    %s\n", hostname)
			}
		}
	}
	fmt.Fprintln(writer, "</pre>")
	fmt.Fprintln(writer, "<h3>")
	for _, htmlWriter := range s.htmlWriters {
		htmlWriter.WriteHtml(writer)
	}
	fmt.Fprintln(writer, "</h3>")
	fmt.Fprintln(writer, "<hr>")
	html.WriteFooter(writer)
	fmt.Fprintln(writer, "</body>")
}
