package httpd

import (
	"bufio"
	"fmt"
	"net/http"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/html"
)

func (s state) showStreamDashboardHandler(w http.ResponseWriter,
	req *http.Request) {
	status := s.unpacker.GetStatus()
	streamName := req.URL.RawQuery
	stream, ok := status.ImageStreams[streamName]
	if !ok {
		http.NotFound(w, req)
		return
	}
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	fmt.Fprintf(writer, "<title>%s status page</title>\n", streamName)
	fmt.Fprintln(writer, `<style>
                          table, th, td {
                          border-collapse: collapse;
                          }
                          </style>`)
	fmt.Fprintln(writer, "<body>")
	fmt.Fprintln(writer, "<center>")
	fmt.Fprintf(writer, "<h1>%s status page</h1>\n", streamName)
	fmt.Fprintln(writer, "</center>")
	fmt.Fprintf(writer, "<b>Device Id:</b> %s<br>\n", stream.DeviceId)
	if device, ok := status.Devices[stream.DeviceId]; ok {
		fmt.Fprintf(writer, "<b>Device Name:</b> %s<br>\n", device.DeviceName)
		fmt.Fprintf(writer, "<b>Device Size:</b> %s<br>\n",
			format.FormatBytes(device.Size))
	}
	fmt.Fprintf(writer, "<b>Status:</b> %s<br>\n",
		s.getStreamStatusLink(streamName, stream, true))
	s.unpacker.WriteStreamHtml(writer, streamName)
	fmt.Fprintln(writer, "<hr>")
	html.WriteFooter(writer)
	fmt.Fprintln(writer, "</body>")
}
