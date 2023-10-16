package httpd

import (
	"bufio"
	"io"
	"net"
	"net/http"

	"github.com/Cloud-Foundations/Dominator/lib/url"
)

func (s state) showLastPatchLogHandler(w http.ResponseWriter,
	req *http.Request) {
	parsedQuery := url.ParseQuery(req.URL)
	if len(parsedQuery.Flags) != 1 {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var ipAddr string
	for name := range parsedQuery.Flags {
		ipAddr = name
	}
	r, _, _, err := s.manager.GetVmLastPatchLog(net.ParseIP(ipAddr))
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer r.Close()
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	io.Copy(writer, r)
}
