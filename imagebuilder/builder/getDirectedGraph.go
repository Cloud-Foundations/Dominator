package builder

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	proto "github.com/Cloud-Foundations/Dominator/proto/imaginator"
)

func (b *Builder) dependencyGeneratorLoop(
	generateDependencyTrigger <-chan struct{}) {
	interval := time.Hour // The first configuration load should happen first.
	timer := time.NewTimer(interval)
	for {
		select {
		case <-generateDependencyTrigger:
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
		}
		startTime := time.Now()
		dependencyData, err := b.generateDependencyData()
		finishTime := time.Now()
		timeTaken := finishTime.Sub(startTime)
		if err != nil {
			b.logger.Printf("failed to generate dependencies: %s\n", err)
		} else {
			b.logger.Debugf(0, "generated dependencies in: %s\n",
				format.Duration(timeTaken))
		}
		b.dependencyDataLock.Lock()
		if dependencyData != nil || b.dependencyData == nil {
			b.dependencyData = dependencyData
			b.dependencyDataError = err
		}
		b.dependencyDataAttempt = finishTime
		b.dependencyDataLock.Unlock()
		interval = timeTaken * 10
		if interval < 10*time.Second {
			interval = 10 * time.Second
		}
		for keepDraining := true; keepDraining; {
			select {
			case <-generateDependencyTrigger:
			default:
				keepDraining = false
			}
		}
		timer.Reset(interval)
	}
}

func (b *Builder) generateDependencyData() (*dependencyDataType, error) {
	var directoriesToRemove []string
	defer func() {
		for _, directory := range directoriesToRemove {
			os.RemoveAll(directory)
		}
	}()
	streamToSource := make(map[string]string) // K: stream name, V: source.
	urlToDirectory := make(map[string]string)
	fetchLog := bytes.NewBuffer(nil)
	streamNames := b.listNormalStreamNames()
	for _, streamName := range streamNames {
		b.streamsLock.RLock()
		stream := b.imageStreams[streamName]
		b.streamsLock.RUnlock()
		if stream == nil {
			return nil, fmt.Errorf("stream: %s does not exist", streamName)
		}
		manifestLocation := stream.getManifestLocation(b, nil)
		var directory string
		if rootDir, ok := urlToDirectory[manifestLocation.url]; ok {
			directory = filepath.Join(rootDir, manifestLocation.directory)
		} else if rootDir, err := urlToLocal(manifestLocation.url); err != nil {
			return nil, err
		} else if rootDir != "" {
			directory = filepath.Join(rootDir, manifestLocation.directory)
		} else {
			gitRoot, err := makeTempDirectory("",
				strings.Replace(streamName, "/", "_", -1)+".manifest")
			if err != nil {
				return nil, err
			}
			directoriesToRemove = append(directoriesToRemove, gitRoot)
			err = gitShallowClone(gitRoot, manifestLocation.url,
				stream.ManifestUrl, "master", []string{"**/manifest"}, fetchLog)
			if err != nil {
				return nil, err
			}
			urlToDirectory[manifestLocation.url] = gitRoot
			directory = filepath.Join(gitRoot, manifestLocation.directory)
		}
		manifestConfig, err := readManifestFile(directory, stream)
		if err != nil {
			return nil, err
		}
		streamToSource[streamName] = manifestConfig.SourceImage
	}
	return &dependencyDataType{
		fetchLog:       fetchLog.Bytes(),
		generatedAt:    time.Now(),
		streamToSource: streamToSource,
	}, nil
}

func (b *Builder) getDependencyData(maxAge time.Duration) (
	*dependencyDataType, error) {
	if maxAge <= 0 {
		maxAge = time.Minute
	} else if maxAge < 2*time.Second {
		maxAge = 2 * time.Second
	}
	for {
		b.dependencyDataLock.RLock()
		dependencyData := b.dependencyData
		lastAttempt := b.dependencyDataAttempt
		err := b.dependencyDataError
		b.dependencyDataLock.RUnlock()
		if time.Since(lastAttempt) < maxAge {
			return dependencyData, err
		}
		b.generateDependencyTrigger <- struct{}{} // Trigger and wait.
	}
}

func (b *Builder) getDirectedGraph() (proto.GetDirectedGraphResult, error) {
	var zero proto.GetDirectedGraphResult
	dependencyData, err := b.getDependencyData(0)
	if err != nil {
		return zero, err
	}
	buffer := bytes.NewBuffer(nil)
	streamNames := make([]string, 0, len(dependencyData.streamToSource))
	for streamName := range dependencyData.streamToSource {
		streamNames = append(streamNames, streamName)
	}
	sort.Strings(streamNames) // For consistent output.
	fmt.Fprintln(buffer, "digraph all {")
	for _, streamName := range streamNames {
		fmt.Fprintf(buffer, "  \"%s\" -> \"%s\"\n",
			streamName, dependencyData.streamToSource[streamName])
	}
	fmt.Fprintln(buffer, "}")
	return proto.GetDirectedGraphResult{
		FetchLog:    dependencyData.fetchLog,
		GraphvizDot: buffer.Bytes(),
	}, nil
}

func (b *Builder) triggerDependencyDataGeneration() {
	select {
	case b.generateDependencyTrigger <- struct{}{}:
	default:
	}
}
