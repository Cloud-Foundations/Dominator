package allocator

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/html"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
)

const commonStyleSheet string = `<style>
table, th, td {
border-collapse: collapse;
}
</style>
`

func (m *Manager) httpSetup() {
	html.HandleFunc("/listAllocationQueue", m.listAllocationQueueHandler)
	html.HandleFunc("/showAllocationRequest", m.showAllocationRequestHandler)
}

func (m *Manager) listAllocationQueueHandler(w http.ResponseWriter,
	req *http.Request) {
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	fmt.Fprintf(writer, "<title>Allocation request queue</title>\n")
	writer.WriteString(commonStyleSheet)
	fmt.Fprintln(writer, "<body>")
	fmt.Fprintln(writer, "<h2>Request Queue:</h2>")
	m.showRequestQueue(writer)
	fmt.Fprintln(writer, "<p><h2>Unfulfilled Allocations:</h2>")
	m.showAllocations(writer)
	fmt.Fprintln(writer, "<p><h2>Old Requests:</h2>")
	m.showDeletions(writer)
	fmt.Fprintln(writer, "</body>")
}

func (m *Manager) showAllocationRequestHandler(w http.ResponseWriter,
	req *http.Request) {
	request := m.getRequest(proto.RequestId(req.URL.RawQuery))
	if request == nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	json.WriteWithIndent(writer, "    ", request)
}

func (m *Manager) showDeletions(writer io.Writer) {
	var deletions []proto.AllocationUpdateEntry
	m.updateQueue.IterateValuesReverse(
		func(entry proto.AllocationUpdateEntry) bool {
			if entry.Deleted != nil {
				deletions = append(deletions, entry)
			}
			if len(deletions) > 100 { // Show at most 100 deletions.
				return false
			}
			return true
		})
	fmt.Fprintln(writer, `<table border="1" style="width:100%">`)
	tw, _ := html.NewTableWriter(writer, true,
		"RequestID",
		"Username",
		"Reason",
		"Timestamp",
	)
	for _, entry := range deletions {
		var reason string
		if entry.Deleted.Error != "" {
			reason = entry.Deleted.Error
		} else {
			reason = entry.Deleted.Reason.String()
		}
		tw.WriteRow("", "",
			fmt.Sprintf("<a href=\"showAllocationRequest?%s\">%s</a>",
				entry.RequestId, entry.RequestId),
			string(entry.Username),
			reason,
			entry.Timestamp.Format(format.TimeFormatSeconds),
		)
	}
	tw.Close()
}

func (m *Manager) showAllocations(writer io.Writer) {
	allocations := m.listAllocations()
	sort.Slice(allocations, func(i, j int) bool {
		return allocations[i].allocation.CreateDeadline.Before(
			allocations[j].allocation.CreateDeadline)
	})
	fmt.Fprintln(writer, `<table border="1" style="width:100%">`)
	tw, _ := html.NewTableWriter(writer, true,
		"RequestID",
		"Username",
		"Num VMs",
		"State",
	)
	now := time.Now()
	for _, allocation := range allocations {
		var state string
		state = fmt.Sprintf("allocated for %s",
			format.Duration(allocation.allocation.CreateDeadline.Sub(now)))
		tw.WriteRow("", "",
			fmt.Sprintf("<a href=\"showAllocationRequest?%s\">%s</a>",
				allocation.requestId, allocation.requestId),
			string(allocation.username),
			fmt.Sprintf("%d", len(allocation.allocation.VMs)),
			state,
		)
	}
	tw.Close()
}

func (m *Manager) showRequestQueue(writer io.Writer) {
	queue := m.listQueue()
	fmt.Fprintln(writer, `<table border="1" style="width:100%">`)
	tw, _ := html.NewTableWriter(writer, true,
		"RequestID",
		"Username",
		"Num VMs",
		"State",
	)
	for _, queueEntry := range queue {
		var state string
		if queueEntry.Request.Deadline.IsZero() {
			state = "waiting"
		} else {
			state = fmt.Sprintf("waiting for %s",
				format.Duration(time.Until(queueEntry.Request.Deadline)))
		}
		tw.WriteRow("", "",
			fmt.Sprintf("<a href=\"showAllocationRequest?%s\">%s</a>",
				queueEntry.RequestId, queueEntry.RequestId),
			string(queueEntry.Username),
			fmt.Sprintf("%d", len(queueEntry.Request.VMs)),
			state,
		)
	}
	tw.Close()
}

func (m *Manager) writeHtml(writer io.Writer) {
	lastLostHeartbeatTime := m.lastLostHeartbeatTime
	if m.lostHeartbeat {
		fmt.Fprintf(writer,
			"<font color=\"red\">Lost allocator heartbeat since %s (%s ago)</font><p>",
			lastLostHeartbeatTime.Format(format.TimeFormatSeconds),
			format.Duration(time.Since(lastLostHeartbeatTime)))
	} else if !lastLostHeartbeatTime.IsZero() {
		fmt.Fprintf(writer,
			"<font color=\"salmon\">Previously lost allocator heartbeat at %s (%s ago)</font><p>",
			lastLostHeartbeatTime.Format(format.TimeFormatSeconds),
			format.Duration(time.Since(lastLostHeartbeatTime)))
	}
	if m.active {
		fmt.Fprintln(writer,
			"Allocation queue <a href=\"listAllocationQueue\">dashboard</a><br>")
	}
}
