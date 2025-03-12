package httpd

import (
	"bufio"
	"fmt"
	"net/http"

	"github.com/Cloud-Foundations/Dominator/lib/html"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

func statusHandler(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		http.NotFound(w, req)
		return
	}
	bd, _ := html.CreateBenchmarkData()
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	fmt.Fprintln(writer, "<title>imageserver status page</title>")
	fmt.Fprintln(writer, "<body>")
	fmt.Fprintln(writer, "<center>")
	fmt.Fprintln(writer, "<h1>imageserver status page</h1>")
	if !srpc.CheckTlsRequired() {
		fmt.Fprintln(writer,
			`<h1><font color="red">Running in insecure mode. Your images can be pwned!!!</font></h1>`)
	}
	fmt.Fprintln(writer, "</center>")
	html.WriteHeaderWithRequestNoGC(writer, req)
	fmt.Fprintln(writer, "<h3>")
	for _, htmlWriter := range htmlWriters {
		htmlWriter.WriteHtml(writer)
	}
	fmt.Fprintln(writer, "</h3>")
	fmt.Fprintln(writer, "<hr>")
	html.WriteFooter(writer)
	if bd != nil {
		bd.Write(writer)
	}
	fmt.Fprintln(writer, "</body>")
}
