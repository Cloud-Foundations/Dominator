package httpd

import (
	"bufio"
	"fmt"
	"net/http"
	"sort"
)

func (s state) listVolumeDirectoriesHandler(w http.ResponseWriter,
	req *http.Request) {
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	volumeDirectories := s.manager.ListVolumeDirectories()
	sort.Strings(volumeDirectories)
	for _, volumeDirectory := range volumeDirectories {
		fmt.Fprintln(writer, volumeDirectory)
	}
}
