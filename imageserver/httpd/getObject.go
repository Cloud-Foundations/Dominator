package httpd

import (
	"io"
	"net/http"
	"strconv"

	"github.com/Cloud-Foundations/Dominator/lib/hash"
)

func (s state) getObjectHandler(w http.ResponseWriter, req *http.Request) {
	var hashVal hash.Hash
	if err := hashVal.UnmarshalText([]byte(req.URL.RawQuery)); err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	size, readCloser, err := s.objectServer.GetObject(hashVal)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer readCloser.Close()
	w.Header().Set("Content-Length", strconv.FormatUint(size, 10))
	io.Copy(w, readCloser)
}
