package httpd

import (
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/Cloud-Foundations/Dominator/imagebuilder/builder"
	"github.com/Cloud-Foundations/Dominator/imagebuilder/logarchiver"
	"github.com/Cloud-Foundations/Dominator/lib/html"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

type HtmlWriter interface {
	WriteHtml(writer io.Writer)
}

type Options struct {
	PortNumber uint
}

type Params struct {
	Builder          *builder.Builder
	BuildLogReporter logarchiver.BuildLogReporter
	DaemonMode       bool
	Logger           log.DebugLogger
}

type state struct {
	builder          *builder.Builder
	buildLogReporter logarchiver.BuildLogReporter
	logger           log.DebugLogger
}

var htmlWriters []HtmlWriter

func StartServer(portNum uint, builderObj *builder.Builder,
	logReporter logarchiver.BuildLogReporter, daemon bool) error {
	return StartServerWithOptionsAndParams(
		Options{PortNumber: portNum},
		Params{Builder: builderObj},
	)
}

func StartServerWithOptionsAndParams(options Options, params Params) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", options.PortNumber))
	if err != nil {
		return err
	}
	myState := state{params.Builder, params.BuildLogReporter, params.Logger}
	html.HandleFunc("/", myState.statusHandler)
	html.HandleFunc("/showCurrentBuildLog", myState.showCurrentBuildLogHandler)
	html.HandleFunc("/showDirectedGraph", myState.showDirectedGraphHandler)
	html.HandleFunc("/showImageStream", myState.showImageStreamHandler)
	html.HandleFunc("/showImageStreams", myState.showImageStreamsHandler)
	html.HandleFunc("/showLastBuildLog", myState.showLastBuildLogHandler)
	if myState.buildLogReporter != nil {
		html.HandleFunc("/showBuildLog", myState.showBuildLogHandler)
		html.HandleFunc("/showBuildLogArchive",
			myState.showBuildLogArchiveHandler)
		html.HandleFunc("/showStreamAllBuilds",
			myState.showStreamAllBuildsHandler)
		html.HandleFunc("/showStreamGoodBuilds",
			myState.showStreamGoodBuildsHandler)
		html.HandleFunc("/showStreamErrorBuilds",
			myState.showStreamErrorBuildsHandler)
	}
	if params.DaemonMode {
		go http.Serve(listener, nil)
	} else {
		http.Serve(listener, nil)
	}
	return nil
}

func AddHtmlWriter(htmlWriter HtmlWriter) {
	htmlWriters = append(htmlWriters, htmlWriter)
}
