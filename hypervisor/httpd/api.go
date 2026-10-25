package httpd

import (
	"io"

	"github.com/Cloud-Foundations/Dominator/hypervisor/manager"
	"github.com/Cloud-Foundations/Dominator/lib/html"
)

type HtmlWriter interface {
	WriteHtml(writer io.Writer)
}

var htmlWriters []HtmlWriter

type state struct {
	manager *manager.Manager
}

// RegisterHandlers registers the HTML handlers on http.DefaultServeMux.
func RegisterHandlers(managerObj *manager.Manager) {
	myState := state{managerObj}
	html.HandleFunc("/", myState.statusHandler)
	html.HandleFunc("/listAvailableAddresses",
		myState.listAvailableAddressesHandler)
	html.HandleFunc("/listVolumeDirectories",
		myState.listVolumeDirectoriesHandler)
	html.HandleFunc("/listRegisteredAddresses",
		myState.listRegisteredAddressesHandler)
	html.HandleFunc("/listSubnets", myState.listSubnetsHandler)
	html.HandleFunc("/listVMs", myState.listVMsHandler)
	html.HandleFunc("/showVmBootLog", myState.showBootLogHandler)
	html.HandleFunc("/showVmLastPatchLog", myState.showLastPatchLogHandler)
	html.HandleFunc("/showVM", myState.showVMHandler)
	html.HandleFunc("/showVolumeDirectories",
		myState.showVolumeDirectoriesHandler)
}

func AddHtmlWriter(htmlWriter HtmlWriter) {
	htmlWriters = append(htmlWriters, htmlWriter)
}
