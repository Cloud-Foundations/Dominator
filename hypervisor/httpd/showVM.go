package httpd

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/html"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/url"
)

var timeFormat string = "02 Jan 2006 15:04:05.99 MST"

func (s state) formatImage(imageName string) string {
	imageServer := s.manager.GetImageServerAddress()
	if imageServer == "" {
		return imageName
	}
	return fmt.Sprintf(`<a href="http://%s/showImage?%s">%s</a>`,
		imageServer, imageName, imageName)
}

func (s state) showVMHandler(w http.ResponseWriter, req *http.Request) {
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
	netIpAddr := net.ParseIP(ipAddr)
	vm, err := s.manager.GetVmInfo(netIpAddr)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	if parsedQuery.OutputType() == url.OutputTypeJson {
		json.WriteWithIndent(writer, "    ", vm)
	} else {
		var storage uint64
		volumeSizes := make([]string, 0, len(vm.Volumes))
		for _, volume := range vm.Volumes {
			storage += volume.Size
			volumeSizes = append(volumeSizes, format.FormatBytes(volume.Size))
		}
		var tagNames []string
		for name := range vm.Tags {
			tagNames = append(tagNames, name)
		}
		sort.Strings(tagNames)
		fmt.Fprintf(writer, "<title>Information for VM %s</title>\n", ipAddr)
		fmt.Fprintln(writer, `<style>
                          table, th, td {
                          border-collapse: collapse;
                          }
                          </style>`)
		fmt.Fprintln(writer, "<body>")
		if lw, _ := s.manager.GetVmLockWatcher(netIpAddr); lw != nil {
			if wroteSomething, _ := lw.WriteHtml(writer, ""); wroteSomething {
				fmt.Fprintln(writer, "<br>")
			}
		}
		fmt.Fprintln(writer, `<table border="0">`)
		if len(vm.Address.IpAddress) < 1 {
			writeString(writer, "IP Address", ipAddr+" (externally allocated)")
		} else if vm.Uncommitted {
			writeString(writer, "IP Address", ipAddr+" (uncommitted)")
		} else {
			writeString(writer, "IP Address", ipAddr)
		}
		if vm.Hostname != "" {
			writeString(writer, "Hostname", vm.Hostname)
		}
		writeString(writer, "MAC Address", vm.Address.MacAddress)
		if vm.ImageName != "" {
			image := fmt.Sprintf("<a href=\"http://%s/showImage?%s\">%s</a>",
				s.manager.GetImageServerAddress(), vm.ImageName, vm.ImageName)
			writeString(writer, "Boot image", image)
		} else if vm.ImageURL != "" {
			writeString(writer, "Boot image URL", vm.ImageURL)
		} else {
			writeString(writer, "Boot image", "was streamed in")
		}
		writeTime(writer, "Created on", vm.CreatedOn)
		writeTime(writer, "Last state change", vm.ChangedStateOn)
		writeString(writer, "State", vm.State.String())
		writeString(writer, "RAM", format.FormatBytes(vm.MemoryInMiB<<20))
		writeString(writer, "CPU", format.FormatMilli(uint64(vm.MilliCPUs)))
		writeStrings(writer, "Volume sizes", volumeSizes)
		writeString(writer, "Total storage", format.FormatBytes(storage))
		writeStrings(writer, "Owner groups", vm.OwnerGroups)
		writeStrings(writer, "Owner users", vm.OwnerUsers)
		if vm.IdentityName != "" {
			writeString(writer, "Identity name",
				fmt.Sprintf("%s, %s",
					vm.IdentityName, makeExpiration(vm.IdentityExpires)))
		}
		writeBool(writer, "Spread volumes", vm.SpreadVolumes)
		writeString(writer, "Latest boot",
			fmt.Sprintf("<a href=\"showVmBootLog?%s\">log</a>", ipAddr))
		rc, size, lastPatchTime, err := s.manager.GetVmLastPatchLog(netIpAddr)
		if err == nil {
			rc.Close()
			writeString(writer, "Last patch",
				fmt.Sprintf(
					"<a href=\"showVmLastPatchLog?%s\">log</a> (%s, at: %s, age: %s)",
					ipAddr, format.FormatBytes(size),
					lastPatchTime.Format(timeFormat),
					format.Duration(time.Since(lastPatchTime))))
		}
		if ok, _ := s.manager.CheckVmHasHealthAgent(netIpAddr); ok {
			writeString(writer, "Health Agent",
				fmt.Sprintf("<a href=\"http://%s:6910/\">detected</a>",
					ipAddr))
		}
		fmt.Fprintln(writer, "</table>")
		fmt.Fprintln(writer, "<br>Tags:<br>")
		fmt.Fprintln(writer, `<table border="1">`)
		tw, _ := html.NewTableWriter(writer, true, "Name", "Value")
		for _, name := range tagNames {
			value := vm.Tags[name]
			switch name {
			case "RequiredImage", "PlannedImage":
				value = s.formatImage(value)
			}
			tw.WriteRow("", "", name, value)
		}
		tw.Close()
		fmt.Fprintln(writer, "<br>")
		fmt.Fprintf(writer,
			"<a href=\"showVM?%s&output=json\">VM info:</a><br>\n",
			vm.Address.IpAddress)
		fmt.Fprintln(writer, `<pre style="background-color: #eee; border: 1px solid #999; display: block; float: left;">`)
		json.WriteWithIndent(writer, "    ", vm)
		fmt.Fprintln(writer, `</pre><p style="clear: both;">`)
		fmt.Fprintln(writer, "</body>")
	}
}

func makeExpiration(value time.Time) string {
	expiresIn := time.Until(value)
	if expiresIn >= 0 {
		return fmt.Sprintf("expires at %s (in %s)",
			value, format.Duration(expiresIn))
	}
	return fmt.Sprintf("expired at %s (%s ago)",
		value, format.Duration(-expiresIn))
}

func writeBool(writer io.Writer, name string, value bool) {
	fmt.Fprintf(writer, "  <tr><td>%s</td><td>%t</td></tr>\n", name, value)
}

func writeInt(writer io.Writer, name string, value int) {
	fmt.Fprintf(writer, "  <tr><td>%s</td><td>%d</td></tr>\n", name, value)
}

func writeString(writer io.Writer, name, value string) {
	fmt.Fprintf(writer, "  <tr><td>%s</td><td>%s</td></tr>\n", name, value)
}

func writeStrings(writer io.Writer, name string, value []string) {
	if len(value) < 1 {
		return
	}
	fmt.Fprintf(writer, "  <tr><td>%s</td><td>%s</td></tr>\n",
		name, strings.Join(value, ", "))
}

func writeTime(writer io.Writer, name string, value time.Time) {
	if value.IsZero() {
		return
	}
	fmt.Fprintf(writer, "  <tr><td>%s</td><td>%s (%s ago)</td></tr>\n",
		name, value.Format(timeFormat), format.Duration(time.Since(value)))
}

func writeUint64(writer io.Writer, name string, value uint64) {
	fmt.Fprintf(writer, "  <tr><td>%s</td><td>%d</td></tr>\n", name, value)
}
