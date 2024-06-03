package httpd

import (
	"bufio"
	"net/http"

	"github.com/Cloud-Foundations/Dominator/hypervisor/manager"
	"github.com/Cloud-Foundations/Dominator/lib/json"
)

type volumeEntry struct {
	Directory string
	manager.VolumeInfo
}

func (s state) showVolumeDirectoriesHandler(w http.ResponseWriter,
	req *http.Request) {
	volumeDirectories := s.manager.ListVolumeDirectories()
	volumeInfos := s.manager.GetVolumeDirectories()
	volumeEntries := make([]volumeEntry, 0, len(volumeDirectories))
	for _, directory := range volumeDirectories {
		if _, ok := volumeInfos[directory]; !ok {
			continue
		}
		volumeEntries = append(volumeEntries, volumeEntry{
			Directory:  directory,
			VolumeInfo: volumeInfos[directory],
		})
	}
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	json.WriteWithIndent(writer, "    ", volumeEntries)
}
