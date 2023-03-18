package httpd

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"

	"github.com/Cloud-Foundations/Dominator/imagebuilder/logarchiver"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/html"
)

type userBuildCountType struct {
	summary  *logarchiver.RequestorSummary
	username string
}

func showBuildInfos(writer io.Writer, buildInfos *logarchiver.BuildInfos,
	showError bool) {
	fmt.Fprintln(writer, `<style>
                          table, th, td {
                          border-collapse: collapse;
                          }
                          </style>`)
	fmt.Fprintln(writer, "<body>")
	fmt.Fprintln(writer, "<h3>")
	imageNames := buildInfos.ImagesByAge
	if len(imageNames) < len(buildInfos.Builds) {
		imageNames = make([]string, 0, len(buildInfos.Builds))
		for imageName := range buildInfos.Builds {
			imageNames = append(imageNames, imageName)
		}
		sort.Strings(imageNames)
	}
	fmt.Fprintln(writer, `<table border="1">`)
	columns := []string{"Image Name", "Build log", "Duration"}
	if showError {
		columns = append(columns, "Error")
	}
	tw, _ := html.NewTableWriter(writer, true, columns...)
	for _, imageName := range imageNames {
		buildInfo := buildInfos.Builds[imageName]
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

func (s state) showAllBuildsHandler(w http.ResponseWriter, req *http.Request) {
	s.showBuildsHandler(w, req, true, true)
}

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
	fmt.Fprintln(writer, "Build summary per image stream:<br>")
	var numBuilds, numGoodBuilds, numErrorBuilds uint64
	streamNames := make([]string, 0, len(summary.Streams))
	for streamName, streamSummary := range summary.Streams {
		numBuilds += streamSummary.NumBuilds
		numGoodBuilds += streamSummary.NumGoodBuilds
		numErrorBuilds += streamSummary.NumErrorBuilds
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
	tw.WriteRow("", "",
		"<b>TOTAL</b>",
		fmt.Sprintf("<a href=\"showAllBuilds\">%d</a>", numBuilds),
		fmt.Sprintf("<a href=\"showGoodBuilds\">%d</a>", numGoodBuilds),
		fmt.Sprintf("<a href=\"showErrorBuilds\">%d</a>", numErrorBuilds))
	fmt.Fprintln(writer, "</table>")
	fmt.Fprintln(writer, "<p>")
	fmt.Fprintln(writer, "Build summary per requestor:<br>")
	userBuildCounts := make([]userBuildCountType, 0, len(summary.Requestors))
	for username, requestorSummary := range summary.Requestors {
		userBuildCounts = append(userBuildCounts,
			userBuildCountType{requestorSummary, username})
	}
	sort.Slice(userBuildCounts, func(i, j int) bool {
		return userBuildCounts[i].summary.NumBuilds >
			userBuildCounts[j].summary.NumBuilds
	})
	fmt.Fprintln(writer, `<table border="1">`)
	tw, _ = html.NewTableWriter(writer, true, "Username", "Num Builds",
		"Num Good", "Num Bad")
	for _, userBuildCount := range userBuildCounts {
		if userBuildCount.username == "" {
			userBuildCount.username = "imaginator"
		}
		tw.WriteRow("", "",
			userBuildCount.username,
			fmt.Sprintf("<a href=\"showRequestorAllBuilds?%s\">%d</a>",
				userBuildCount.username, userBuildCount.summary.NumBuilds),
			fmt.Sprintf("<a href=\"showRequestorGoodBuilds?%s\">%d</a>",
				userBuildCount.username, userBuildCount.summary.NumGoodBuilds),
			fmt.Sprintf("<a href=\"showRequestorErrorBuilds?%s\">%d</a>",
				userBuildCount.username, userBuildCount.summary.NumErrorBuilds))
	}
	fmt.Fprintln(writer, "</table>")
	fmt.Fprintln(writer, "</body>")
}

func (s state) showBuildsHandler(w http.ResponseWriter,
	req *http.Request, showGood bool, showError bool) {
	logType := "none"
	if showGood && !showError {
		logType = "good"
	} else if showGood && showError {
		logType = "all"
	} else if showError {
		logType = "bad"
	}
	buildInfos := s.buildLogReporter.GetBuildInfos(showGood, showError)
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	fmt.Fprintf(writer, "<title>build log archive (%s)</title>", logType)
	showBuildInfos(writer, buildInfos, showError)
}

func (s state) showGoodBuildsHandler(w http.ResponseWriter, req *http.Request) {
	s.showBuildsHandler(w, req, true, false)
}

func (s state) showErrorBuildsHandler(w http.ResponseWriter,
	req *http.Request) {
	s.showBuildsHandler(w, req, false, true)
}

func (s state) showStreamAllBuildsHandler(w http.ResponseWriter,
	req *http.Request) {
	s.showStreamBuildsHandler(w, req, true, true)
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
	showBuildInfos(writer, buildInfos, showError)
}

func (s state) showStreamGoodBuildsHandler(w http.ResponseWriter,
	req *http.Request) {
	s.showStreamBuildsHandler(w, req, true, false)
}

func (s state) showStreamErrorBuildsHandler(w http.ResponseWriter,
	req *http.Request) {
	s.showStreamBuildsHandler(w, req, false, true)
}

func (s state) showRequestorAllBuildsHandler(w http.ResponseWriter,
	req *http.Request) {
	s.showRequestorBuildsHandler(w, req, true, true)
}

func (s state) showRequestorBuildsHandler(w http.ResponseWriter,
	req *http.Request, showGood bool, showError bool) {
	logType := "none"
	if showGood && !showError {
		logType = "good"
	} else if showGood && showError {
		logType = "all"
	} else if showError {
		logType = "bad"
	}
	username := filepath.Clean(req.URL.RawQuery)
	if username == "imaginator" {
		username = ""
	}
	buildInfos := s.buildLogReporter.GetBuildInfosForRequestor(username,
		showGood, showError)
	if buildInfos == nil {
		http.NotFound(w, req)
		return
	}
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	fmt.Fprintf(writer, "<title>build log archive (%s)</title>", logType)
	showBuildInfos(writer, buildInfos, showError)
}

func (s state) showRequestorGoodBuildsHandler(w http.ResponseWriter,
	req *http.Request) {
	s.showRequestorBuildsHandler(w, req, true, false)
}

func (s state) showRequestorErrorBuildsHandler(w http.ResponseWriter,
	req *http.Request) {
	s.showRequestorBuildsHandler(w, req, false, true)
}
