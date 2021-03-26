package builder

import (
	"bytes"
	"fmt"
	"os"
)

func (b *Builder) getDirectedGraph() ([]byte, error) {
	buffer := bytes.NewBuffer(nil)
	streamNames := b.listNormalStreamNames()
	fmt.Fprintln(buffer, "digraph all {")
	buildLog := new(bytes.Buffer)
	for _, streamName := range streamNames {
		b.streamsLock.RLock()
		stream := b.imageStreams[streamName]
		b.streamsLock.RUnlock()
		if stream == nil {
			return nil, fmt.Errorf("stream: %s does not exist", streamName)
		}
		dirname, sourceImage, _, _, _, err := stream.getSourceImage(b, buildLog)
		if err != nil {
			return nil, fmt.Errorf("error getting manifest for: %s: %s",
				streamName, err)
		}
		os.RemoveAll(dirname)
		fmt.Fprintf(buffer, "  \"%s\" -> \"%s\"\n", streamName, sourceImage)
	}
	fmt.Fprintln(buffer, "}")
	return buffer.Bytes(), nil
}
