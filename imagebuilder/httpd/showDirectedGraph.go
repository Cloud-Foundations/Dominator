package httpd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/html"
	proto "github.com/Cloud-Foundations/Dominator/proto/imaginator"
)

var processStartTime = time.Now()

func (s state) showDirectedGraphHandler(w http.ResponseWriter,
	req *http.Request) {
	if req.Method == "POST" {
		s.builder.GetDirectedGraph(proto.GetDirectedGraphRequest{
			MaxAge: 2 * time.Second,
		})
		http.Redirect(w, req, "/showDirectedGraph", http.StatusFound)
	}
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
	queries := req.URL.Query()
	s.writeDirectedGraph(writer, queries["exclude"], queries["include"])
	fmt.Fprintln(writer, "<hr>")
	html.WriteFooter(writer)
	fmt.Fprintln(writer, "</body>")
}

func (s state) writeDirectedGraph(writer io.Writer,
	excludes, includes []string) {
	result, err := s.builder.GetDirectedGraph(
		proto.GetDirectedGraphRequest{Excludes: excludes, Includes: includes})
	if err != nil {
		fmt.Fprintf(writer, "error getting graph data: %s<br>\n", err)
		return
	}
	if result.GeneratedAt.IsZero() { // No data yet.
		fmt.Fprintln(writer, "No data generated yet<br>")
		if time.Since(processStartTime) > 2*time.Second {
			fmt.Fprintln(writer,
				`<form enctype="application/x-www-form-urlencoded" action="/showDirectedGraph" method="post">`)
			fmt.Fprintln(writer,
				`<input type="submit" value="Regenerate">`)
		}
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
	weblinks, _ := s.builder.GetRelationshipsQuickLinks()
	for _, weblink := range weblinks {
		fmt.Fprintf(writer,
			"<a href=\"/showDirectedGraph%s\">%s</a>&nbsp;&nbsp;&nbsp;\n",
			weblink.URL, weblink.Name)
	}
	if len(weblinks) > 0 {
		fmt.Fprintln(writer, "<br>")
	}
	if len(result.FetchLog) > 0 {
		fmt.Fprintln(writer,
			"<hr style=\"height:2px\"><font color=\"#bbb\">")
		fmt.Fprintln(writer, "<b>Fetch log:</b>")
		fmt.Fprintln(writer, "<pre>")
		writer.Write(result.FetchLog)
		fmt.Fprintln(writer, "</pre>")
		fmt.Fprintln(writer, "</font>")
	}
	fmt.Fprintf(writer, "Data generated at: %s<br>\n",
		result.GeneratedAt.Format(format.TimeFormatSeconds))
	if result.LastAttemptError != "" {
		fmt.Fprintf(writer,
			"Last generation attempt at: %s failed: %s<br>\n",
			result.LastAttemptAt.Format(format.TimeFormatSeconds),
			result.LastAttemptError)
	}
	if time.Since(result.GeneratedAt) > 2*time.Second {
		fmt.Fprintln(writer,
			`<form enctype="application/x-www-form-urlencoded" action="/showDirectedGraph" method="post">`)
		fmt.Fprintln(writer,
			`<input type="submit" value="Regenerate">`)
	}
}
