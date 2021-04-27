package builder

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (b *Builder) getDirectedGraph() ([]byte, error) {
	var directoriesToRemove []string
	defer func() {
		for _, directory := range directoriesToRemove {
			os.RemoveAll(directory)
		}
	}()
	urlToDirectory := make(map[string]string)
	buffer := bytes.NewBuffer(nil)
	streamNames := b.listNormalStreamNames()
	sort.Strings(streamNames) // For consistent output.
	fmt.Fprintln(buffer, "digraph all {")
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
			err = gitShallowClone(gitRoot, manifestLocation.url, "master",
				[]string{"**/manifest"}, ioutil.Discard)
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
		fmt.Fprintf(buffer, "  \"%s\" -> \"%s\"\n",
			streamName, manifestConfig.SourceImage)
	}
	fmt.Fprintln(buffer, "}")
	return buffer.Bytes(), nil
}
