package main

import (
	"fmt"
	"io"
)

func (imageObjectServers *imageObjectServersType) WriteHtml(writer io.Writer) {
	imageObjectServers.objSrv.WriteHtml(writer)
	if imageObjectServers.imageServerAddress != "" {
		fmt.Fprintf(writer,
			"Replication master: <a href=\"http://%s/\">%s</a><br>\n",
			imageObjectServers.imageServerAddress,
			imageObjectServers.imageServerAddress)
	}
}
