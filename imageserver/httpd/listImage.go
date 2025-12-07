package httpd

import (
	"bufio"
	"fmt"
	"net/http"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
)

func printHashLink(hashVal hash.Hash, size uint64) string {
	if size > 1<<20 {
		return fmt.Sprintf("%x", hashVal)
	}
	return fmt.Sprintf("<a href=\"getObject?%x\">%x</a>",
		hashVal, hashVal)
}

func (s state) listImageHandler(w http.ResponseWriter, req *http.Request) {
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	imageName := req.URL.RawQuery
	fmt.Fprintf(writer, "<title>image %s</title>\n", imageName)
	fmt.Fprintln(writer, "<body>")
	fmt.Fprintln(writer, "<h3>")
	image := s.imageDataBase.GetImage(imageName)
	if image == nil {
		fmt.Fprintf(writer, "Image: %s UNKNOWN!\n", imageName)
	} else {
		fmt.Fprintf(writer, "File-system data for image: %s\n", imageName)
		fmt.Fprintln(writer, "</h3>")
		fmt.Fprintln(writer, "<pre>")
		var listParams filesystem.ListParams
		if s.allowUnauthenticatedReads {
			listParams.FormatHash = printHashLink
		}
		image.FileSystem.ListWithParams(writer, listParams)
		fmt.Fprintln(writer, "</pre>")
	}
	fmt.Fprintln(writer, "</body>")
}
