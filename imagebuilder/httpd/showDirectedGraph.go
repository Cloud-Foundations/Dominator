package httpd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os/exec"

	"github.com/Cloud-Foundations/Dominator/lib/html"
	proto "github.com/Cloud-Foundations/Dominator/proto/imaginator"
)

func (s state) showDirectedGraphHandler(w http.ResponseWriter,
	req *http.Request) {
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	fmt.Fprintln(writer, "<title>imaginator image stream relationshops</title>")
	fmt.Fprintln(writer, `<style>
                          table, th, td {
                          border-collapse: collapse;
                          }
                          </style>`)
	fmt.Fprintln(writer, "<body>")
	fmt.Fprintln(writer, "<center>")
	fmt.Fprintln(writer, "<h1>imaginator image stream relationships</h1>")
	fmt.Fprintln(writer, "</center>")
	s.writeDirectedGraph(writer)
	fmt.Fprintln(writer, "<hr>")
	html.WriteFooter(writer)
	fmt.Fprintln(writer, "</body>")
}

func (s state) writeDirectedGraph(writer io.Writer) {
	result, err := s.builder.GetDirectedGraph(proto.GetDirectedGraphRequest{})
	if err != nil {
		fmt.Fprintf(writer, "error getting graph data: %s<br>\n", err)
		return
	}
	cmd := exec.Command("dot", "-Tsvg")
	cmd.Stdin = bytes.NewReader(result.GraphvizDot)
	cmd.Stdout = writer
	cmd.Stderr = writer
	err = cmd.Run()
	if err == nil {
		fmt.Fprintln(writer, "<p>")
	} else {
		fmt.Fprintf(writer, "error rendering graph: %s<br>\n", err)
		fmt.Fprintln(writer, "Showing graph data:<br>")
		fmt.Fprintln(writer, "<pre>")
		writer.Write(result.GraphvizDot)
		fmt.Fprintln(writer, "</pre>")
	}
	if len(result.FetchLog) > 0 {
		fmt.Fprintln(writer, "<hr style=\"height:2px\"><font color=\"#bbb\">")
		fmt.Fprintln(writer, "<b>Fetch log:</b>")
		fmt.Fprintln(writer, "<pre>")
		writer.Write(result.FetchLog)
		fmt.Fprintln(writer, "</pre>")
		fmt.Fprintln(writer, "</font>")
	}
}
