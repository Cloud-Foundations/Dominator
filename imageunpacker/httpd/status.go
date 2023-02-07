package httpd

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"sort"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/html"
	proto "github.com/Cloud-Foundations/Dominator/proto/imageunpacker"
)

func getStreamDashboardLink(streamName string, ok bool) string {
	if !ok {
		return ""
	}
	return fmt.Sprintf("<a href=\"showStreamDashboard?%s\">%s</a>",
		streamName, streamName)
}

func (s state) getStreamStatusLink(streamName string,
	stream proto.ImageStreamInfo, ok bool) string {
	if !ok {
		return stream.Status.String()
	}
	fs, _ := s.unpacker.GetFileSystem(streamName)
	if fs == nil {
		return stream.Status.String()
	}
	return fmt.Sprintf("<a href=\"showFileSystem?%s\">%s</a>",
		streamName, stream.Status.String())
}

func (s state) statusHandler(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		http.NotFound(w, req)
		return
	}
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	fmt.Fprintln(writer, "<title>image-unpacker status page</title>")
	fmt.Fprintln(writer, `<style>
                          table, th, td {
                          border-collapse: collapse;
                          }
                          </style>`)
	fmt.Fprintln(writer, "<body>")
	fmt.Fprintln(writer, "<center>")
	fmt.Fprintln(writer, "<h1>image-unpacker status page</h1>")
	fmt.Fprintln(writer, "</center>")
	html.WriteHeaderWithRequest(writer, req)
	fmt.Fprintln(writer, "<h3>")
	s.writeDashboard(writer)
	for _, htmlWriter := range htmlWriters {
		htmlWriter.WriteHtml(writer)
	}
	fmt.Fprintln(writer, "</h3>")
	fmt.Fprintln(writer, "<hr>")
	html.WriteFooter(writer)
	fmt.Fprintln(writer, "</body>")
}

func (s state) writeDashboard(writer io.Writer) {
	status := s.unpacker.GetStatus()
	fmt.Fprintln(writer, "Image streams:<br>")
	fmt.Fprintln(writer, `<table border="1">`)
	tw, _ := html.NewTableWriter(writer, true,
		"Image Stream", "Device Id", "Device Name", "Size", "Status")
	streamNames := make([]string, 0, len(status.ImageStreams))
	for streamName := range status.ImageStreams {
		streamNames = append(streamNames, streamName)
	}
	sort.Strings(streamNames)
	for _, streamName := range streamNames {
		stream := status.ImageStreams[streamName]
		tw.WriteRow("", "",
			getStreamDashboardLink(streamName, true),
			stream.DeviceId,
			status.Devices[stream.DeviceId].DeviceName,
			func() string {
				size := status.Devices[stream.DeviceId].Size
				if size < 1 {
					return ""
				}
				return format.FormatBytes(size)
			}(),
			s.getStreamStatusLink(streamName, stream, true),
		)
	}
	fmt.Fprintln(writer, "</table><br>")
	fmt.Fprintln(writer, "Devices:<br>")
	fmt.Fprintln(writer, `<table border="1">`)
	tw, _ = html.NewTableWriter(writer, true, "Device Id", "Device Name",
		"Size", "Image Stream", "Status")
	deviceIds := make([]string, 0, len(status.Devices))
	for deviceId := range status.Devices {
		deviceIds = append(deviceIds, deviceId)
	}
	sort.Strings(deviceIds)
	for _, deviceId := range deviceIds {
		deviceInfo := status.Devices[deviceId]
		stream, ok := status.ImageStreams[deviceInfo.StreamName]
		tw.WriteRow("", "",
			deviceId,
			deviceInfo.DeviceName,
			format.FormatBytes(deviceInfo.Size),
			getStreamDashboardLink(deviceInfo.StreamName, ok),
			s.getStreamStatusLink(deviceInfo.StreamName, stream, ok),
		)
	}
	fmt.Fprintln(writer, "</table><br>")
}
