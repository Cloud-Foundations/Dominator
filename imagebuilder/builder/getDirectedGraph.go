package builder

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/concurrent"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	proto "github.com/Cloud-Foundations/Dominator/proto/imaginator"
)

func isExcluded(streamName string, excludes []string) bool {
	for _, exclude := range excludes {
		if len(streamName) < len(exclude) {
			continue
		}
		if streamName == exclude {
			return true
		}
		if len(streamName) <= len(exclude) {
			continue
		}
		if streamName[len(exclude)] != '/' {
			continue
		}
		if strings.HasPrefix(streamName, exclude) {
			return true
		}
	}
	return false
}

// computeExcludes will compute the set of excluded image streams. Dependent
// streams are also excluded.
func computeExcludes(streamToSource map[string]string,
	bootstrapStreams []string, excludes []string) map[string]struct{} {
	if len(excludes) < 1 {
		return nil
	}
	allStreams := make(map[string]struct{})
	excludedStreams := make(map[string]struct{})
	streamToDependents := make(map[string][]string)
	for _, stream := range bootstrapStreams {
		allStreams[stream] = struct{}{}
	}
	for stream, source := range streamToSource {
		allStreams[stream] = struct{}{}
		streamToDependents[source] = append(streamToDependents[source], stream)
	}
	for streamName := range allStreams {
		if isExcluded(streamName, excludes) {
			walkDependents(streamToDependents, streamName, func(name string) {
				excludedStreams[name] = struct{}{}
			})
		}
	}
	return excludedStreams
}

func walkDependents(streamToDependents map[string][]string, streamName string,
	fn func(string)) {
	for _, name := range streamToDependents[streamName] {
		walkDependents(streamToDependents, name, fn)
	}
	fn(streamName)
}

func (b *Builder) dependencyGeneratorLoop(
	generateDependencyTrigger <-chan chan<- struct{}) {
	interval := time.Hour // The first configuration load should happen first.
	timer := time.NewTimer(interval)
	for {
		var wakeChannel chan<- struct{}
		select {
		case wakeChannel = <-generateDependencyTrigger:
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
		}
		startTime := time.Now()
		dependencyData, fetchTime, err := b.generateDependencyData()
		finishTime := time.Now()
		timeTaken := finishTime.Sub(startTime)
		if err != nil {
			b.logger.Printf("failed to generate dependencies: %s\n", err)
		} else {
			b.logger.Debugf(0, "generated dependencies in: %s (fetch: %s)\n",
				format.Duration(timeTaken), fetchTime)
		}
		b.dependencyDataLock.Lock()
		if dependencyData != nil {
			b.dependencyData = dependencyData
		}
		b.dependencyDataAttempt = finishTime
		b.dependencyDataError = err
		b.dependencyDataLock.Unlock()
		if wakeChannel != nil {
			wakeChannel <- struct{}{}
		}
		interval = fetchTime * 10
		if interval < 10*time.Second {
			interval = 10 * time.Second
		}
		for keepDraining := true; keepDraining; {
			select {
			case wakeChannel := <-generateDependencyTrigger:
				if wakeChannel != nil {
					wakeChannel <- struct{}{}
				}
			default:
				keepDraining = false
			}
		}
		timer.Reset(interval)
	}
}

func (b *Builder) generateDependencyData() (
	*dependencyDataType, time.Duration, error) {
	var directoriesToRemove []string
	defer func() {
		for _, directory := range directoriesToRemove {
			os.RemoveAll(directory)
		}
	}()
	streamToSource := make(map[string]string) // K: stream name, V: source.
	urlToDirectory := make(map[string]string)
	streamNames := b.listNormalStreamNames()
	startTime := time.Now()
	streams := make(map[string]*imageStreamType, len(streamNames))
	// First pass to process local manifests and start Git fetches.
	state := concurrent.NewState(0)
	var lock sync.Mutex
	fetchLog := bytes.NewBuffer(nil)
	var serialisedFetchTime time.Duration
	for _, streamName := range streamNames {
		b.streamsLock.RLock()
		stream := b.imageStreams[streamName]
		b.streamsLock.RUnlock()
		if stream == nil {
			return nil, 0, fmt.Errorf("stream: %s does not exist", streamName)
		}
		streams[streamName] = stream
		manifestLocation := stream.getManifestLocation(b, nil)
		if _, ok := urlToDirectory[manifestLocation.url]; ok {
			continue // Git fetch has started.
		} else if rootDir, err := urlToLocal(manifestLocation.url); err != nil {
			return nil, 0, err
		} else if rootDir != "" {
			manifestConfig, err := readManifestFile(
				filepath.Join(rootDir, manifestLocation.directory), stream)
			if err != nil {
				return nil, 0, err
			}
			streamToSource[streamName] = manifestConfig.SourceImage
			delete(streams, streamName) // Mark as completed.
		} else {
			gitRoot, err := makeTempDirectory("",
				strings.Replace(streamName, "/", "_", -1)+".manifest")
			if err != nil {
				return nil, 0, err
			}
			directoriesToRemove = append(directoriesToRemove, gitRoot)
			err = state.GoRun(func() error {
				myFetchLog := bytes.NewBuffer(nil)
				startTime := time.Now()
				err := gitShallowClone(gitRoot, manifestLocation.url,
					stream.ManifestUrl, "master", []string{"**/manifest"},
					myFetchLog)
				lock.Lock()
				fetchLog.Write(myFetchLog.Bytes())
				serialisedFetchTime += time.Since(startTime)
				lock.Unlock()
				return err
			})
			urlToDirectory[manifestLocation.url] = gitRoot // Mark fetch started
		}
	}
	if err := state.Reap(); err != nil {
		return nil, 0, err
	}
	// Second pass to process fetched manifests.
	for streamName, stream := range streams {
		manifestLocation := stream.getManifestLocation(b, nil)
		manifestConfig, err := readManifestFile(
			filepath.Join(urlToDirectory[manifestLocation.url],
				manifestLocation.directory), stream)
		if err != nil {
			return nil, 0, err
		}
		streamToSource[streamName] = manifestConfig.SourceImage
	}
	fmt.Fprintf(fetchLog, "Cumulative fetch time: %s\n",
		format.Duration(serialisedFetchTime))
	unbuildableSources := make(map[string]struct{})
	for streamName, sourceName := range streamToSource {
		if _, ok := streamToSource[sourceName]; ok {
			continue
		}
		if b.getBootstrapStream(sourceName) != nil {
			continue
		}
		unbuildableSources[sourceName] = struct{}{}
		if b.getNumBootstrapStreams() > 0 {
			b.logger.Printf("stream: %s has unbuildable source: %s\n",
				streamName, sourceName)
		}
	}
	finishTime := time.Now()
	fmt.Fprintf(fetchLog, "Generated dependencies in: %s\n",
		format.Duration(finishTime.Sub(startTime)))
	return &dependencyDataType{
		fetchLog:           fetchLog.Bytes(),
		generatedAt:        finishTime,
		streamToSource:     streamToSource,
		unbuildableSources: unbuildableSources,
	}, serialisedFetchTime, nil
}

func (b *Builder) getDependencies(request proto.GetDependenciesRequest) (
	proto.GetDependenciesResult, error) {
	dependencyData, lastAttempt, lastErr := b.getDependencyData(request.MaxAge)
	if dependencyData == nil {
		return proto.GetDependenciesResult{
			LastAttemptAt:    lastAttempt,
			LastAttemptError: errors.ErrorToString(lastErr),
		}, nil
	}
	return proto.GetDependenciesResult{
		FetchLog:           dependencyData.fetchLog,
		GeneratedAt:        dependencyData.generatedAt,
		LastAttemptAt:      lastAttempt,
		LastAttemptError:   errors.ErrorToString(lastErr),
		StreamToSource:     dependencyData.streamToSource,
		UnbuildableSources: dependencyData.unbuildableSources,
	}, nil

}

// getDependencyData returns the dependency data (possibly stale or nil), the
// time of the last attempt to generate and the error result for the last
// attempt. If maxAge is larger than zero, getDependencyData will wait until
// there is an attempt less than maxAge ago.
func (b *Builder) getDependencyData(maxAge time.Duration) (
	*dependencyDataType, time.Time, error) {
	if maxAge <= 0 {
		b.dependencyDataLock.RLock()
		defer b.dependencyDataLock.RUnlock()
		return b.dependencyData, b.dependencyDataAttempt, b.dependencyDataError
	}
	if maxAge < 2*time.Second {
		maxAge = 2 * time.Second
	}
	for {
		b.dependencyDataLock.RLock()
		dependencyData := b.dependencyData
		lastAttempt := b.dependencyDataAttempt
		err := b.dependencyDataError
		b.dependencyDataLock.RUnlock()
		if time.Since(lastAttempt) < maxAge {
			return dependencyData, lastAttempt, err
		}
		waitChannel := make(chan struct{}, 1)
		b.generateDependencyTrigger <- waitChannel // Trigger and wait.
		<-waitChannel
	}
}

func (b *Builder) getDirectedGraph(request proto.GetDirectedGraphRequest) (
	proto.GetDirectedGraphResult, error) {
	dependencyData, lastAttempt, lastErr := b.getDependencyData(request.MaxAge)
	if dependencyData == nil {
		return proto.GetDirectedGraphResult{
			LastAttemptAt:    lastAttempt,
			LastAttemptError: errors.ErrorToString(lastErr),
		}, nil
	}
	bootstrapStreams := b.listBootstrapStreamNames()
	excludedStreams := computeExcludes(dependencyData.streamToSource,
		bootstrapStreams, request.Excludes)
	streamNames := make([]string, 0, len(dependencyData.streamToSource))
	for streamName := range dependencyData.streamToSource {
		if _, ok := excludedStreams[streamName]; !ok {
			streamNames = append(streamNames, streamName)
		}
	}
	sort.Strings(streamNames) // For consistent output.
	buffer := bytes.NewBuffer(nil)
	fmt.Fprintln(buffer, "digraph all {")
	for _, streamName := range streamNames {
		fmt.Fprintf(buffer, "  \"%s\" -> \"%s\"\n",
			streamName, dependencyData.streamToSource[streamName])
	}
	// Mark streams with no source in red, to show they are unbuildable.
	for streamName := range dependencyData.unbuildableSources {
		if _, ok := excludedStreams[streamName]; !ok {
			fmt.Fprintf(buffer, "  \"%s\" [fontcolor=red]\n", streamName)
		}
	}
	// Mark streams which are auto rebuilt in bold.
	for _, streamName := range bootstrapStreams {
		if _, ok := excludedStreams[streamName]; !ok {
			fmt.Fprintf(buffer, "  \"%s\" [style=bold]\n", streamName)
		}
	}
	for _, streamName := range b.imageStreamsToAutoRebuild {
		if _, ok := excludedStreams[streamName]; !ok {
			fmt.Fprintf(buffer, "  \"%s\" [style=bold]\n", streamName)
		}
	}
	fmt.Fprintln(buffer, "}")
	return proto.GetDirectedGraphResult{
		FetchLog:         dependencyData.fetchLog,
		GeneratedAt:      dependencyData.generatedAt,
		GraphvizDot:      buffer.Bytes(),
		LastAttemptAt:    lastAttempt,
		LastAttemptError: errors.ErrorToString(lastErr),
	}, nil
}

func (b *Builder) triggerDependencyDataGeneration() {
	select {
	case b.generateDependencyTrigger <- nil:
	default:
	}
}
