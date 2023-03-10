package logarchiver

import (
	"io"
	"os"
	"path/filepath"
)

func (a *buildLogArchiver) GetBuildInfosForStream(streamName string,
	includeGood, includeBad bool) *BuildInfos {
	buildInfos := &BuildInfos{make(map[string]BuildInfo)}
	a.mutex.Lock()
	defer a.mutex.Unlock()
	for name, image := range a.imageStreams[streamName].images {
		if (includeGood && image.buildInfo.Error != "") &&
			includeBad && image.buildInfo.Error == "" {
			continue
		}
		buildInfos.Builds[filepath.Join(streamName, name)] = image.buildInfo
	}
	return buildInfos
}

func (a *buildLogArchiver) GetBuildLog(imageName string) (
	io.ReadCloser, error) {
	return os.Open(filepath.Join(a.options.Topdir, imageName, "buildLog"))
}

func (a *buildLogArchiver) GetSummary() *Summary {
	summary := &Summary{make(map[string]*StreamSummary)}
	a.mutex.Lock()
	defer a.mutex.Unlock()
	for _, imageStream := range a.imageStreams {
		streamSummary := StreamSummary{}
		for _, image := range imageStream.images {
			streamSummary.NumBuilds++
			if image.buildInfo.Error == "" {
				streamSummary.NumGoodBuilds++
			} else {
				streamSummary.NumErrorBuilds++
			}
		}
		summary.Streams[imageStream.name] = &streamSummary
	}
	return summary
}
