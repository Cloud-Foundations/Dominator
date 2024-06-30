package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/Cloud-Foundations/Dominator/lib/html"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
)

type HtmlWriter interface {
	WriteHtml(writer io.Writer)
}

type httpServer struct {
	htmlWriters []HtmlWriter
	mdb         *mdb.Mdb
	generators  *generatorList
}

func makeNumMachinesText(numFilteredMachines, numRawMachines uint) string {
	if numRawMachines == numFilteredMachines {
		return fmt.Sprintf("%d", numFilteredMachines)
	}
	return fmt.Sprintf("%d<font color=\"grey\">/%d</font>",
		numFilteredMachines, numRawMachines)
}

func startHttpServer(portNum uint,
	generators *generatorList) (*httpServer, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", portNum))
	if err != nil {
		return nil, err
	}
	s := &httpServer{mdb: &mdb.Mdb{}, generators: generators}
	html.HandleFunc("/", s.statusHandler)
	html.HandleFunc("/showMdb", s.showMdbHandler)
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
			makeNumMachinesText(numFilteredMachines, numRawMachines))
		tw.WriteRow("", "", columns...)
	}
	columns := make([]string, s.generators.maxArgs+2)
	columns[0] = "<b>TOTAL</b>"
	columns[s.generators.maxArgs+1] = makeNumMachinesText(totalFilteredMachines,
		totalRawMachines)
	tw.WriteRow("", "", columns...)
	tw.Close()
	fmt.Fprintf(writer, "Number of machines: <a href=\"showMdb\">%d</a><br>\n",
		len(s.mdb.Machines))
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

func (s *httpServer) showMdbHandler(w http.ResponseWriter, req *http.Request) {
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	json.WriteWithIndent(writer, "    ", s.mdb)
}

func (s *httpServer) UpdateMdb(new *mdb.Mdb) {
	if new == nil {
		new = &mdb.Mdb{}
	}
	s.mdb = new
}
