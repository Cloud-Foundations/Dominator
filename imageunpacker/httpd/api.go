package httpd

import (
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/Cloud-Foundations/Dominator/imageunpacker/unpacker"
	"github.com/Cloud-Foundations/Dominator/lib/html"
)

type HtmlWriter interface {
	WriteHtml(writer io.Writer)
}

var htmlWriters []HtmlWriter

type state struct {
	unpacker *unpacker.Unpacker
}

func StartServer(portNum uint, unpackerObj *unpacker.Unpacker,
	daemon bool) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", portNum))
	if err != nil {
		return err
	}
	myState := state{unpackerObj}
	html.HandleFunc("/", myState.statusHandler)
	html.HandleFunc("/showFileSystem", myState.showFileSystemHandler)
	html.HandleFunc("/showStreamDashboard", myState.showStreamDashboardHandler)
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
