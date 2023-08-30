package html

import (
	"bufio"
	"io"
	"net/http"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

type BenchmarkData struct {
	startRusage wsyscall.Rusage
	startTime   time.Time
}
type HtmlWriter interface {
	WriteHtml(writer io.Writer)
}

type TableWriter struct {
	doHighlighting bool
	lastBackground string
	writer         io.Writer
}

func BenchmarkedHandler(handler func(io.Writer,
	*http.Request)) func(http.ResponseWriter, *http.Request) {
	return benchmarkedHandler(handler)
}

func CreateBenchmarkData() (*BenchmarkData, error) {
	return createBenchmarkData()
}

func (bd *BenchmarkData) Write(w *bufio.Writer) error {
	return bd.write(w)
}

func HandleFunc(pattern string,
	handler func(w http.ResponseWriter, req *http.Request)) {
	handleFunc(http.DefaultServeMux, pattern, handler)
}

func RegisterHtmlWriterForPattern(pattern, title string,
	htmlWriter HtmlWriter) {
	registerHtmlWriterForPattern(pattern, title, htmlWriter)
}

func ServeMuxHandleFunc(serveMux *http.ServeMux, pattern string,
	handler func(w http.ResponseWriter, req *http.Request)) {
	handleFunc(serveMux, pattern, handler)
}

func SetSecurityHeaders(w http.ResponseWriter) {
	setSecurityHeaders(w)
}

func NewTableWriter(writer io.Writer, doHighlighting bool,
	columns ...string) (*TableWriter, error) {
	return newTableWriter(writer, doHighlighting, columns)
}

func (tw *TableWriter) CloseRow() error {
	return tw.closeRow()
}

func (tw *TableWriter) OpenRow(foreground, background string) error {
	return tw.openRow(foreground, background)
}

func (tw *TableWriter) WriteData(foreground, data string) error {
	return tw.writeData(foreground, data)
}

func (tw *TableWriter) WriteRow(foreground, background string,
	columns ...string) error {
	return tw.writeRow(foreground, background, columns)
}

func WriteFooter(writer io.Writer) {
	writeFooter(writer)
}

func WriteHeader(writer io.Writer) {
	writeHeader(writer, nil, false)
}

func WriteHeaderNoGC(writer io.Writer) {
	writeHeader(writer, nil, true)
}

func WriteHeaderWithRequest(writer io.Writer, req *http.Request) {
	writeHeader(writer, req, false)
}

func WriteHeaderWithRequestNoGC(writer io.Writer, req *http.Request) {
	writeHeader(writer, req, true)
}
