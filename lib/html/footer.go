package html

import (
	"fmt"
	"io"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/version"
)

func writeFooter(writer io.Writer) {
	info := version.Get()
	fmt.Fprintf(writer, "Page generated at: %s with %s (%s)<br>\n",
		time.Now().Format(format.TimeFormatSeconds),
		info.Version, info.GoVersion)
}
