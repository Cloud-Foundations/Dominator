package filter

import (
	"fmt"
	"io"

	"github.com/Cloud-Foundations/Dominator/lib/pathregexp"
)

const (
	colour      = "#ff6000"
	colourName  = "dark orange"
	htmlMessage = `Unoptimised regular expressions are shown in <font color="` +
		colour + `">` + colourName + `</font>.<br>`
)

func (filter *Filter) writeHtml(writer io.Writer) {
	if filter.matchers == nil {
		if err := filter.Compile(); err != nil {
			fmt.Fprintln(writer, err)
			return
		}
	}
	var showLegend bool
	for index, line := range filter.FilterLines {
		if ok := pathregexp.IsOptimised(filter.matchers[index]); ok {
			fmt.Fprintf(writer, "<code>%s</code><br>\n", line)
		} else {
			fmt.Fprintf(writer,
				"<font color=\"%s\"><code>%s</code></font><br>\n", colour, line)
			showLegend = true
		}
	}
	if !showLegend {
		return
	}
	fmt.Fprintln(writer, "<h4>")
	fmt.Fprintln(writer, htmlMessage)
	fmt.Fprintln(writer, "</h4>")
}
