package httpd

import (
	"bufio"
	"net/http"

	"github.com/Cloud-Foundations/Dominator/lib/json"
)

func (s state) showVolumeDirectoriesHandler(w http.ResponseWriter,
	req *http.Request) {
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	volumeDirectories := s.manager.GetVolumeDirectories()
	json.WriteWithIndent(writer, "    ", volumeDirectories)
}
