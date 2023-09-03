package logarchiver

import (
	"io"
	"os"
	"path/filepath"
)

func (a *buildLogArchiver) GetBuildInfos(includeGood bool,
	includeBad bool) *BuildInfos {
	buildInfos := &BuildInfos{Builds: make(map[string]BuildInfo)}
	a.mutex.Lock()
	defer a.mutex.Unlock()
	for element := a.ageList.Front(); element != nil; element = element.Next() {
		image := element.Value.(*imageType)
		imageStream := image.imageStream
		if image.buildInfo.Error == "" && !includeGood {
			continue
		}
		if image.buildInfo.Error != "" && !includeBad {
			continue
		}
		imageName := filepath.Join(imageStream.name, image.name)
		buildInfos.Builds[imageName] = image.buildInfo
		buildInfos.ImagesByAge = append(buildInfos.ImagesByAge, imageName)
	}
	return buildInfos
}

func (a *buildLogArchiver) GetBuildInfosForRequestor(username string,
	includeGood, includeBad bool) *BuildInfos {
	buildInfos := &BuildInfos{Builds: make(map[string]BuildInfo)}
	a.mutex.Lock()
	defer a.mutex.Unlock()
	for _, imageStream := range a.imageStreams {
		for name, image := range imageStream.images {
			if username != image.buildInfo.RequestorUsername {
				continue
			}
			if image.buildInfo.Error == "" && !includeGood {
				continue
			}
			if image.buildInfo.Error != "" && !includeBad {
				continue
			}
			buildInfos.Builds[filepath.Join(imageStream.name,
				name)] = image.buildInfo
		}
	}
	return buildInfos
}

func (a *buildLogArchiver) GetBuildInfosForStream(streamName string,
	includeGood, includeBad bool) *BuildInfos {
	buildInfos := &BuildInfos{Builds: make(map[string]BuildInfo)}
	a.mutex.Lock()
	defer a.mutex.Unlock()
	for name, image := range a.imageStreams[streamName].images {
		if image.buildInfo.Error == "" && !includeGood {
			continue
		}
		if image.buildInfo.Error != "" && !includeBad {
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
	summary := &Summary{
		Streams:    make(map[string]*StreamSummary),
		Requestors: make(map[string]*RequestorSummary),
	}
	a.mutex.Lock()
	defer a.mutex.Unlock()
	for _, imageStream := range a.imageStreams {
		streamSummary := StreamSummary{}
		for _, image := range imageStream.images {
			requestorSummary :=
				summary.Requestors[image.buildInfo.RequestorUsername]
			if requestorSummary == nil {
				requestorSummary = &RequestorSummary{}
				summary.Requestors[image.buildInfo.RequestorUsername] =
					requestorSummary
			}
			requestorSummary.NumBuilds++
			streamSummary.NumBuilds++
			if image.buildInfo.Error == "" {
				requestorSummary.NumGoodBuilds++
				streamSummary.NumGoodBuilds++
			} else {
				requestorSummary.NumErrorBuilds++
				streamSummary.NumErrorBuilds++
			}
		}
		summary.Streams[imageStream.name] = &streamSummary
	}
	return summary
}
