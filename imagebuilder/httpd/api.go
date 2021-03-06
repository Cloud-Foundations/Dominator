package httpd

import (
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/Cloud-Foundations/Dominator/imagebuilder/builder"
	"github.com/Cloud-Foundations/Dominator/lib/html"
)

type HtmlWriter interface {
	WriteHtml(writer io.Writer)
}

var htmlWriters []HtmlWriter

type state struct {
	builder *builder.Builder
}

func StartServer(portNum uint, builderObj *builder.Builder,
	daemon bool) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", portNum))
	if err != nil {
		return err
	}
	myState := state{builderObj}
	html.HandleFunc("/", myState.statusHandler)
	html.HandleFunc("/showCurrentBuildLog", myState.showCurrentBuildLogHandler)
	html.HandleFunc("/showImageStream", myState.showImageStreamHandler)
	html.HandleFunc("/showImageStreams", myState.showImageStreamsHandler)
	html.HandleFunc("/showLastBuildLog", myState.showLastBuildLogHandler)
	if daemon {
		go http.Serve(listener, nil)
	} else {
		http.Serve(listener, nil)
	}
	return nil
}

func AddHtmlWriter(htmlWriter HtmlWriter) {
	htmlWriters = append(htmlWriters, htmlWriter)
}
