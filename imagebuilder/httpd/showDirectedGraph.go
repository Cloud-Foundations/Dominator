package httpd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os/exec"

	"github.com/Cloud-Foundations/Dominator/lib/html"
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
	graph, err := s.builder.GetDirectedGraph()
	if err != nil {
		fmt.Fprintf(writer, "error getting graph data: %s<br>\n", err)
		return
	}
	cmd := exec.Command("dot", "-Tsvg")
	cmd.Stdin = bytes.NewReader(graph)
	cmd.Stdout = writer
	cmd.Stderr = writer
	err = cmd.Run()
	if err == nil {
		fmt.Fprintln(writer, "<p>")
		return
	}
	fmt.Fprintf(writer, "error rendering graph: %s<br>\n", err)
	fmt.Fprintln(writer, "Showing graph data:<br>")
	fmt.Fprintln(writer, "<pre>")
	writer.Write(graph)
	fmt.Fprintln(writer, "</pre>")
}
