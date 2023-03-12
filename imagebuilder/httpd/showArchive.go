package httpd

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/html"
)

func (s state) showBuildLogHandler(w http.ResponseWriter,
	req *http.Request) {
	imageName := filepath.Clean(req.URL.RawQuery)
	reader, err := s.buildLogReporter.GetBuildLog(imageName)
	if err != nil {
		http.NotFound(w, req)
		return
	}
	defer reader.Close()
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	io.Copy(writer, reader)
}

func (s state) showBuildLogArchiveHandler(w http.ResponseWriter,
	req *http.Request) {
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	fmt.Fprintln(writer, "<title>build log archive</title>")
	fmt.Fprintln(writer, `<style>
                          table, th, td {
                          border-collapse: collapse;
                          }
                          </style>`)
	fmt.Fprintln(writer, "<body>")
	fmt.Fprintln(writer, "<h3>")
	summary := s.buildLogReporter.GetSummary()
	streamNames := make([]string, 0, len(summary.Streams))
	for streamName := range summary.Streams {
		streamNames = append(streamNames, streamName)
	}
	sort.Strings(streamNames)
	fmt.Fprintln(writer, `<table border="1">`)
	tw, _ := html.NewTableWriter(writer, true,
		"Image Stream", "Num Builds", "Num Good", "Num Bad")
	for _, streamName := range streamNames {
		streamSummary := summary.Streams[streamName]
		tw.WriteRow("", "",
			streamName,
			fmt.Sprintf("<a href=\"showStreamAllBuilds?%s\">%d</a>",
				streamName, streamSummary.NumBuilds),
			fmt.Sprintf("<a href=\"showStreamGoodBuilds?%s\">%d</a>",
				streamName, streamSummary.NumGoodBuilds),
			fmt.Sprintf("<a href=\"showStreamErrorBuilds?%s\">%d</a>",
				streamName, streamSummary.NumErrorBuilds))
	}
	fmt.Fprintln(writer, "</table>")
	fmt.Fprintln(writer, "</body>")
}

func (s state) showStreamAllBuildsHandler(w http.ResponseWriter,
	req *http.Request) {
	s.showStreamBuildsHandler(w, req, true, true)
}

func (s state) showStreamGoodBuildsHandler(w http.ResponseWriter,
	req *http.Request) {
	s.showStreamBuildsHandler(w, req, true, false)
}

func (s state) showStreamErrorBuildsHandler(w http.ResponseWriter,
	req *http.Request) {
	s.showStreamBuildsHandler(w, req, false, true)
}

func (s state) showStreamBuildsHandler(w http.ResponseWriter,
	req *http.Request, showGood bool, showError bool) {
	logType := "none"
	if showGood && !showError {
		logType = "good"
	} else if showGood && showError {
		logType = "all"
	} else if showError {
		logType = "bad"
	}
	streamName := filepath.Clean(req.URL.RawQuery)
	buildInfos := s.buildLogReporter.GetBuildInfosForStream(streamName,
		showGood, showError)
	if buildInfos == nil {
		http.NotFound(w, req)
		return
	}
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	fmt.Fprintf(writer, "<title>build log archive (%s)</title>", logType)
	fmt.Fprintln(writer, `<style>
                          table, th, td {
                          border-collapse: collapse;
                          }
                          </style>`)
	fmt.Fprintln(writer, "<body>")
	fmt.Fprintln(writer, "<h3>")
	imageNames := make([]string, 0, len(buildInfos.Builds))
	for imageName := range buildInfos.Builds {
		imageNames = append(imageNames, imageName)
	}
	sort.Strings(imageNames)
	fmt.Fprintln(writer, `<table border="1">`)
	columns := []string{"Image Name", "Build log", "Duration"}
	if showError {
		columns = append(columns, "Error")
	}
	tw, _ := html.NewTableWriter(writer, true, columns...)
	for _, imageName := range imageNames {
		buildInfo := buildInfos.Builds[imageName]
		if !showGood && buildInfo.Error == "" ||
			!showError && buildInfo.Error != "" {
			continue
		}
		columns := []string{
			imageName,
			fmt.Sprintf("<a href=\"showBuildLog?%s\">log</a>", imageName),
			format.Duration(buildInfo.Duration),
		}
		if showError {
			columns = append(columns, buildInfo.Error)
		}
		tw.WriteRow("", "", columns...)
	}
	fmt.Fprintln(writer, "</table>")
	fmt.Fprintln(writer, "</body>")
}
