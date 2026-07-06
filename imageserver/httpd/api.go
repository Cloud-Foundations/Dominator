package httpd

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"text/template"

	"github.com/Cloud-Foundations/Dominator/imageserver/scanner"
	"github.com/Cloud-Foundations/Dominator/lib/html"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver"
)

type Config struct {
	AllowUnauthenticatedReads   bool
	InformationDatabaseTemplate string
	PortNumber                  uint
}

type HtmlWriter interface {
	WriteHtml(writer io.Writer)
}

type Params struct {
	DaemonMode    bool
	ImageDataBase *scanner.ImageDataBase
	Logger        log.DebugLogger
	ObjectServer  objectserver.ObjectServer
}

var htmlWriters []HtmlWriter

type state struct {
	allowUnauthenticatedReads   bool
	imageDataBase               *scanner.ImageDataBase
	informationDatabaseTemplate *template.Template
	logger                      log.DebugLogger
	objectServer                objectserver.ObjectServer
}

func StartServer(config Config, params Params) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", config.PortNumber))
	if err != nil {
		return err
	}
	myState := state{
		allowUnauthenticatedReads: config.AllowUnauthenticatedReads,
		imageDataBase:             params.ImageDataBase,
		logger:                    params.Logger,
		objectServer:              params.ObjectServer,
	}
	if config.InformationDatabaseTemplate != "" {
		tmpl, err := template.New("").Parse(config.InformationDatabaseTemplate)
		if err != nil {
			return fmt.Errorf("cannot parse template: %s: %s",
				config.InformationDatabaseTemplate, err)
		}
		myState.informationDatabaseTemplate = tmpl
	}
	html.HandleFunc("/", statusHandler)
	if config.AllowUnauthenticatedReads {
		html.HandleFunc("/getObject", myState.getObjectHandler)
	}
	html.HandleFunc("/listBuildLog", myState.listBuildLogHandler)
	html.HandleFunc("/listComputedInodes", myState.listComputedInodesHandler)
	html.HandleFunc("/listDirectories", myState.listDirectoriesHandler)
	html.HandleFunc("/listFilter", myState.listFilterHandler)
	html.HandleFunc("/listImage", myState.listImageHandler)
	html.HandleFunc("/listImages", myState.listImagesHandler)
	html.HandleFunc("/listPackages", myState.listPackagesHandler)
	html.HandleFunc("/listReleaseNotes", myState.listReleaseNotesHandler)
	html.HandleFunc("/listTriggers", myState.listTriggersHandler)
	html.HandleFunc("/showImage", myState.showImageHandler)
	if params.DaemonMode {
		go http.Serve(listener, nil)
	} else {
		http.Serve(listener, nil)
	}
	return nil
}

func AddHtmlWriter(htmlWriter HtmlWriter) {
	if htmlWriter != nil {
		htmlWriters = append(htmlWriters, htmlWriter)
	}
}
