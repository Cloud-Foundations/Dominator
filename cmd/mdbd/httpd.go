package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/html"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
	"github.com/Cloud-Foundations/Dominator/lib/url"
)

type HtmlWriter interface {
	WriteHtml(writer io.Writer)
}

type httpServer struct {
	htmlWriters []HtmlWriter
	mdb         *mdbType
	generators  *generatorList
	pauseTable  *pauseTableType
	variables   map[string]string
}

func makeNumMachinesText(numFilteredMachines, numRawMachines uint) string {
	if numRawMachines == numFilteredMachines {
		return fmt.Sprintf("%d", numFilteredMachines)
	}
	return fmt.Sprintf("%d<font color=\"grey\">/%d</font>",
		numFilteredMachines, numRawMachines)
}

func startHttpServer(portNum uint, variables map[string]string,
	generators *generatorList,
	pauseTable *pauseTableType) (*httpServer, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", portNum))
	if err != nil {
		return nil, err
	}
	s := &httpServer{
		mdb:        &mdbType{},
		generators: generators,
		pauseTable: pauseTable,
		variables:  variables}
	html.HandleFunc("/", s.statusHandler)
	html.HandleFunc("/getVariable", s.getVariableHandler)
	html.HandleFunc("/getVariables", s.getVariablesHandler)
	html.HandleFunc("/showMachine", s.showMachineHandler)
	html.HandleFunc("/showMdb", s.showMdbHandler)
	html.HandleFunc("/showPaused", s.showPausedHandler)
	go http.Serve(listener, nil)
	return s, nil
}

func (s *httpServer) statusHandler(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		http.NotFound(w, req)
		return
	}
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	fmt.Fprintln(writer, "<title>MDB daemon status page</title>")
	fmt.Fprintln(writer, `<style>
	                          table, th, td {
	                          border-collapse: collapse;
	                          }
	                          </style>`)
	fmt.Fprintln(writer, "<body>")
	fmt.Fprintln(writer, "<center>")
	fmt.Fprintln(writer, "<h1><b>MDB daemon</b> status page</h1>")
	fmt.Fprintln(writer, "</center>")
	html.WriteHeaderWithRequestNoGC(writer, req)
	fmt.Fprintln(writer, "<h3>")
	if len(s.variables) > 0 {
		fmt.Fprintln(writer, `<a href="getVariables">Variables</a><br>`)
	}
	fmt.Fprintln(writer, "Data Sources:<br>")
	fieldArgs := []string{"Type"}
	for index := uint(0); index < s.generators.maxArgs; index++ {
		fieldArgs = append(fieldArgs, fmt.Sprintf("Arg%d", index))
	}
	fieldArgs = append(fieldArgs, "Num Machines")
	fmt.Fprintln(writer, `<table border="1" style="width:100%">`)
	var totalFilteredMachines, totalRawMachines uint
	tw, _ := html.NewTableWriter(writer, true, fieldArgs...)
	for _, genInfo := range s.generators.generatorInfos {
		columns := make([]string, 0, len(genInfo.args)+2)
		columns = append(columns, genInfo.driverName)
		columns = append(columns, genInfo.args...)
		genInfo.mutex.Lock()
		numFilteredMachines := genInfo.numFilteredMachines
		numRawMachines := genInfo.numRawMachines
		genInfo.mutex.Unlock()
		totalFilteredMachines += numFilteredMachines
		totalRawMachines += numRawMachines
		columns = append(columns,
			fmt.Sprintf("<a href=\"showMdb?dataSourceType=%s\">%s</a>",
				genInfo.driverName,
				makeNumMachinesText(numFilteredMachines, numRawMachines)))
		tw.WriteRow("", "", columns...)
	}
	columns := make([]string, s.generators.maxArgs+2)
	columns[0] = "<b>TOTAL</b>"
	columns[s.generators.maxArgs+1] = fmt.Sprintf("<a href=\"showMdb\">%s</a>",
		makeNumMachinesText(totalFilteredMachines, totalRawMachines))
	tw.WriteRow("", "", columns...)
	tw.Close()
	fmt.Fprintf(writer, "Number of machines: <a href=\"showMdb\">%d</a> (",
		len(s.mdb.Machines))
	fmt.Fprint(writer, "<a href=\"showMdb?output=json\">JSON</a>")
	fmt.Fprintln(writer, ", <a href=\"showMdb?output=text\">text)</a><br>")
	if pauseTableLength := s.pauseTable.len(); pauseTableLength > 0 {
		fmt.Fprintf(writer,
			"Number of paused machines: <a href=\"showPaused\">%d</a> (",
			pauseTableLength)
		fmt.Fprintf(writer,
			"<a href=\"showPaused?output=json\">JSON</a>")
		fmt.Fprintf(writer,
			", <a href=\"showPaused?output=csv\">CSV</a>")
		fmt.Fprintf(writer,
			", <a href=\"showPaused?output=text\">text</a>")
		fmt.Fprintln(writer, ")<br>")
	}
	for _, htmlWriter := range s.htmlWriters {
		htmlWriter.WriteHtml(writer)
	}
	fmt.Fprintln(writer, "</h3>")
	fmt.Fprintln(writer, "<hr>")
	html.WriteFooter(writer)
	fmt.Fprintln(writer, "</body>")
}

func (s *httpServer) AddHtmlWriter(htmlWriter HtmlWriter) {
	s.htmlWriters = append(s.htmlWriters, htmlWriter)
}

func (s *httpServer) getVariableHandler(w http.ResponseWriter,
	req *http.Request) {
	variableName := req.URL.RawQuery
	if variable, ok := s.variables[variableName]; ok {
		fmt.Fprintln(w, variable)
	} else {
		http.NotFound(w, req)
	}
}

func (s *httpServer) getVariablesHandler(w http.ResponseWriter,
	req *http.Request) {
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	json.WriteWithIndent(writer, "    ", s.variables)
}

func (s *httpServer) showMachineHandler(w http.ResponseWriter,
	req *http.Request) {
	hostname := req.URL.RawQuery
	if machine, ok := s.mdb.table[hostname]; !ok {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		return
	} else {
		writer := bufio.NewWriter(w)
		defer writer.Flush()
		json.WriteWithIndent(writer, "    ", machine)
	}
}

func (s *httpServer) showMdbHandler(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	parsedQuery := url.ParseQuery(req.URL)
	selectedDataSourceType := req.FormValue("dataSourceType")
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	var machines []*mdb.Machine
	if selectedDataSourceType == "" {
		machines = s.mdb.Machines
	} else {
		machines = make([]*mdb.Machine, 0, len(s.mdb.Machines))
		for _, machine := range s.mdb.Machines {
			if selectedDataSourceType == machine.DataSourceType {
				machines = append(machines, machine)
			}
		}
	}
	switch parsedQuery.OutputType() {
	case url.OutputTypeHtml:
		fieldArgs := []string{
			"Hostname",
			"IP Address",
			"Data Source Type",
			"Required Image",
			"Planned Image",
		}
		fmt.Fprintln(writer, "<title>MDB Machines</title>")
		fmt.Fprintln(writer, `<style>
	                          table, th, td {
	                          border-collapse: collapse;
	                          }
	                          </style>`)
		fmt.Fprintln(writer, "<body>")
		fmt.Fprintln(writer, `<table border="1" style="width:100%">`)
		tw, _ := html.NewTableWriter(writer, true, fieldArgs...)
		for _, machine := range machines {
			columns := []string{
				fmt.Sprintf("<a href=\"showMachine?%s\">%s</a>",
					machine.Hostname, machine.Hostname),
				machine.IpAddress,
				fmt.Sprintf("<a href=\"showMdb?dataSourceType=%s\">%s</a>",
					machine.DataSourceType, machine.DataSourceType),
				machine.RequiredImage,
				machine.PlannedImage,
			}
			tw.WriteRow("", "", columns...)
		}
		tw.Close()
		fmt.Fprintln(writer, "</body>")
	case url.OutputTypeJson:
		mdbData := *s.mdb
		mdbData.Machines = machines
		json.WriteWithIndent(writer, "    ", mdbData)
	case url.OutputTypeText:
		for _, machine := range machines {
			fmt.Fprintln(writer, machine.Hostname)
		}
	}
}

func (s *httpServer) showPausedHandler(w http.ResponseWriter,
	req *http.Request) {
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	parsedQuery := url.ParseQuery(req.URL)
	pauseList := s.pauseTable.getEntries()
	sort.SliceStable(pauseList, func(left, right int) bool {
		return pauseList[left].Hostname < pauseList[right].Hostname
	})
	fieldArgs := []string{
		"Hostname",
		"Username",
		"Reason",
		"Until",
		"For"}
	switch parsedQuery.OutputType() {
	case url.OutputTypeHtml:
		fmt.Fprintln(writer, "<title>MDB daemon paused machines</title>")
		fmt.Fprintln(writer, `<style>
	                          table, th, td {
	                          border-collapse: collapse;
	                          }
	                          </style>`)
		fmt.Fprintln(writer, "<body>")
		fmt.Fprintln(writer, `<table border="1" style="width:100%">`)
		tw, _ := html.NewTableWriter(writer, true, fieldArgs...)
		for _, pauseData := range pauseList {
			columns := []string{
				pauseData.Hostname,
				pauseData.Username,
				pauseData.Reason,
				pauseData.Until.String(),
				format.Duration(time.Until(pauseData.Until))}
			tw.WriteRow("", "", columns...)
		}
		tw.Close()
		fmt.Fprintln(writer, "</body>")
	case url.OutputTypeText:
		for _, pauseData := range pauseList {
			fmt.Fprintln(writer, pauseData.Hostname)
		}
	case url.OutputTypeJson:
		json.WriteWithIndent(writer, "    ", pauseList)
	case url.OutputTypeCsv:
		w := csv.NewWriter(writer)
		defer w.Flush()
		w.Write(fieldArgs)
		for _, pauseData := range pauseList {
			w.Write([]string{
				pauseData.Hostname,
				pauseData.Username,
				pauseData.Reason,
				pauseData.Until.String(),
				format.Duration(time.Until(pauseData.Until)),
			})
		}
	}
}

func (s *httpServer) UpdateMdb(new *mdbType) {
	if new == nil {
		new = &mdbType{}
	}
	s.mdb = new
}
