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

type dependencyResultType struct {
	fetchLog           []byte
	fetchTime          time.Duration
	resultTime         time.Time
	streamToSource     map[string]string // K: stream name, V: source stream.
	unbuildableSources map[string]struct{}
}

func isMatch(streamName string, patterns []string) bool {
	for _, pattern := range patterns {
		if len(streamName) < len(pattern) {
			continue
		}
		if streamName == pattern {
			return true
		}
		if len(streamName) <= len(pattern) {
			continue
		}
		if streamName[len(pattern)] != '/' {
			continue
		}
		if strings.HasPrefix(streamName, pattern) {
			return true
		}
	}
	return false
}

// computeExcludes will compute the set of excluded image streams. Dependent
// streams are also excluded.
func computeExcludes(streamToSource map[string]string,
	bootstrapStreams []string, excludes []string,
	includes []string) map[string]struct{} {
	if len(excludes) < 1 && len(includes) < 1 {
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
	if len(excludes) > 0 {
		for streamName := range allStreams {
			if isMatch(streamName, excludes) {
				walkDependents(streamToDependents, streamName,
					func(name string) {
						excludedStreams[name] = struct{}{}
					})
			}
		}
	}
	if len(includes) < 1 {
		return excludedStreams
	}
	includedStreams := make(map[string]struct{})
	for streamName := range allStreams {
		if isMatch(streamName, includes) {
			walkDependents(streamToDependents, streamName,
				func(name string) {
					includedStreams[name] = struct{}{}
				})
			walkParents(streamToSource, streamName,
				func(name string) {
					includedStreams[name] = struct{}{}
				})
		}
	}
	for streamName := range allStreams {
		if _, ok := includedStreams[streamName]; !ok {
			excludedStreams[streamName] = struct{}{}
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

func walkParents(streamToSource map[string]string, streamName string,
	fn func(string)) {
	if name, ok := streamToSource[streamName]; ok {
		walkParents(streamToSource, name, fn)
		fn(name)
	}
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
		dependencyResult, err := b.generateDependencyData()
		if err != nil {
			b.logger.Printf("failed to generate dependencies: %s\n", err)
			dependencyData := dependencyDataType{
				lastAttemptError:    err,
				lastAttemptFetchLog: dependencyResult.fetchLog,
				lastAttemptTime:     dependencyResult.resultTime,
			}
			b.dependencyDataLock.Lock()
			if oldData := b.dependencyData; oldData != nil {
				dependencyData.generatedAt = oldData.generatedAt
				dependencyData.streamToSource = oldData.streamToSource
				dependencyData.unbuildableSources = oldData.unbuildableSources
			}
			b.dependencyData = &dependencyData
			b.dependencyDataLock.Unlock()
		} else {
			dependencyData := dependencyDataType{
				generatedAt:         dependencyResult.resultTime,
				lastAttemptFetchLog: dependencyResult.fetchLog,
				lastAttemptTime:     dependencyResult.resultTime,
				streamToSource:      dependencyResult.streamToSource,
				unbuildableSources:  dependencyResult.unbuildableSources,
			}
			b.dependencyDataLock.Lock()
			b.dependencyData = &dependencyData
			b.dependencyDataLock.Unlock()
		}
		if wakeChannel != nil {
			wakeChannel <- struct{}{}
		}
		interval = dependencyResult.fetchTime * 10
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

func (b *Builder) generateDependencyData() (*dependencyResultType, error) {
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
			return nil, fmt.Errorf("stream: %s does not exist", streamName)
		}
		streams[streamName] = stream
		manifestLocation := stream.getManifestLocation(b, nil)
		if _, ok := urlToDirectory[manifestLocation.url]; ok {
			continue // Git fetch has started.
		} else if rootDir, err := urlToLocal(manifestLocation.url); err != nil {
			return nil, err
		} else if rootDir != "" {
			manifestConfig, err := readManifestFile(
				filepath.Join(rootDir, manifestLocation.directory), stream)
			if err != nil {
				return nil, err
			}
			streamToSource[streamName] = manifestConfig.SourceImage
			delete(streams, streamName) // Mark as completed.
		} else {
			gitRoot, err := makeTempDirectory("",
				strings.Replace(streamName, "/", "_", -1)+".manifest")
			if err != nil {
				return nil, err
			}
			directoriesToRemove = append(directoriesToRemove, gitRoot)
			state.GoRun(func() error {
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
		return &dependencyResultType{
			fetchLog:   fetchLog.Bytes(),
			fetchTime:  serialisedFetchTime,
			resultTime: time.Now(),
		}, err
	}
	// Second pass to process fetched manifests.
	for streamName, stream := range streams {
		manifestLocation := stream.getManifestLocation(b, nil)
		manifestConfig, err := readManifestFile(
			filepath.Join(urlToDirectory[manifestLocation.url],
				manifestLocation.directory), stream)
		if err != nil {
			return nil, err
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
	finishedTime := time.Now()
	timeTaken := format.Duration(finishedTime.Sub(startTime))
	b.logger.Debugf(0, "generated dependencies in: %s (fetch: %s)\n",
		timeTaken, format.Duration(serialisedFetchTime))
	fmt.Fprintf(fetchLog, "Generated dependencies in: %s\n", timeTaken)
	return &dependencyResultType{
		fetchLog:           fetchLog.Bytes(),
		fetchTime:          serialisedFetchTime,
		resultTime:         finishedTime,
		streamToSource:     streamToSource,
		unbuildableSources: unbuildableSources,
	}, nil
}

func (b *Builder) getDependencies(request proto.GetDependenciesRequest) (
	proto.GetDependenciesResult, error) {
	dependencyData := b.getDependencyData(request.MaxAge)
	if dependencyData == nil {
		return proto.GetDependenciesResult{}, nil
	}
	lastErrorString := errors.ErrorToString(dependencyData.lastAttemptError)
	return proto.GetDependenciesResult{
		FetchLog:           dependencyData.lastAttemptFetchLog,
		GeneratedAt:        dependencyData.generatedAt,
		LastAttemptAt:      dependencyData.lastAttemptTime,
		LastAttemptError:   lastErrorString,
		StreamToSource:     dependencyData.streamToSource,
		UnbuildableSources: dependencyData.unbuildableSources,
	}, nil

}

// getDependencyData returns the dependency data (possibly nil). If maxAge is
// larger than zero, getDependencyData will wait until there is an attempt less
// than maxAge ago.
func (b *Builder) getDependencyData(maxAge time.Duration) *dependencyDataType {
	if maxAge <= 0 {
		b.dependencyDataLock.RLock()
		dependencyData := b.dependencyData
		b.dependencyDataLock.RUnlock()
		return dependencyData
	}
	if maxAge < 2*time.Second {
		maxAge = 2 * time.Second
	}
	for {
		b.dependencyDataLock.RLock()
		dependencyData := b.dependencyData
		b.dependencyDataLock.RUnlock()
		if time.Since(dependencyData.lastAttemptTime) < maxAge {
			return dependencyData
		}
		waitChannel := make(chan struct{}, 1)
		b.generateDependencyTrigger <- waitChannel // Trigger and wait.
		<-waitChannel
	}
}

func (b *Builder) getDirectedGraph(request proto.GetDirectedGraphRequest) (
	proto.GetDirectedGraphResult, error) {
	dependencyData := b.getDependencyData(request.MaxAge)
	if dependencyData == nil {
		return proto.GetDirectedGraphResult{}, nil
	}
	lastErrorString := errors.ErrorToString(dependencyData.lastAttemptError)
	if dependencyData.generatedAt.IsZero() {
		return proto.GetDirectedGraphResult{
			FetchLog:         dependencyData.lastAttemptFetchLog,
			LastAttemptAt:    dependencyData.lastAttemptTime,
			LastAttemptError: lastErrorString,
		}, nil
	}
	bootstrapStreams := b.listBootstrapStreamNames()
	excludedStreams := computeExcludes(dependencyData.streamToSource,
		bootstrapStreams, request.Excludes, request.Includes)
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
		FetchLog:         dependencyData.lastAttemptFetchLog,
		GeneratedAt:      dependencyData.generatedAt,
		GraphvizDot:      buffer.Bytes(),
		LastAttemptAt:    dependencyData.lastAttemptTime,
		LastAttemptError: lastErrorString,
	}, nil
}

func (b *Builder) triggerDependencyDataGeneration() {
	select {
	case b.generateDependencyTrigger <- nil:
	default:
	}
}
