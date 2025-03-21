package builder

import (
	"fmt"
)

func (b *Builder) getBootstrapStream(name string) *bootstrapStream {
	b.streamsLock.RLock()
	defer b.streamsLock.RUnlock()
	return b.bootstrapStreams[name]
}

func (b *Builder) getNormalStream(name string) *imageStreamType {
	b.streamsLock.RLock()
	defer b.streamsLock.RUnlock()
	return b.imageStreams[name]
}

func (b *Builder) getNumBootstrapStreams() int {
	b.streamsLock.RLock()
	defer b.streamsLock.RUnlock()
	return len(b.bootstrapStreams)
}

func (b *Builder) getNumNormalStreams() int {
	b.streamsLock.RLock()
	defer b.streamsLock.RUnlock()
	return len(b.imageStreams)
}

func (b *Builder) getNumStreams() int {
	b.streamsLock.RLock()
	defer b.streamsLock.RUnlock()
	return len(b.bootstrapStreams) + len(b.imageStreams)
}

func (b *Builder) getPatternedNormalStream(name string) (
	*imageStreamType, error) {
	var matchingPatterns []*imageStreamPatternType
	b.streamsLock.RLock()
	for _, streamPattern := range b.imageStreamPatterns {
		if streamPattern.regexp.MatchString(name) {
			matchingPatterns = append(matchingPatterns, streamPattern)
		}
	}
	b.streamsLock.RUnlock()
	if len(matchingPatterns) < 1 {
		return nil, nil
	}
	if len(matchingPatterns) > 1 {
		return nil, fmt.Errorf("multiple stream patterns match: \"%s\"", name)
	}
	pattern := matchingPatterns[0]
	return &imageStreamType{
		builder:                      b,
		builderUsers:                 pattern.builderUsers,
		name:                         name,
		imageStreamConfigurationType: pattern.imageStreamConfigurationType,
	}, nil
}

func (b *Builder) listAllStreamNames() []string {
	b.streamsLock.RLock()
	defer b.streamsLock.RUnlock()
	imageStreamNames := make([]string, 0,
		len(b.bootstrapStreams)+len(b.imageStreams))
	for name := range b.bootstrapStreams {
		imageStreamNames = append(imageStreamNames, name)
	}
	for name := range b.imageStreams {
		imageStreamNames = append(imageStreamNames, name)
	}
	return imageStreamNames
}

func (b *Builder) listBootstrapStreamNames() []string {
	b.streamsLock.RLock()
	defer b.streamsLock.RUnlock()
	imageStreamNames := make([]string, 0, len(b.bootstrapStreams))
	for name := range b.bootstrapStreams {
		imageStreamNames = append(imageStreamNames, name)
	}
	return imageStreamNames
}

func (b *Builder) listNormalStreamNames() []string {
	b.streamsLock.RLock()
	defer b.streamsLock.RUnlock()
	imageStreamNames := make([]string, 0, len(b.imageStreams))
	for name := range b.imageStreams {
		imageStreamNames = append(imageStreamNames, name)
	}
	return imageStreamNames
}

func (b *Builder) listStreamsToAutoRebuild() []string {
	b.streamsLock.RLock()
	defer b.streamsLock.RUnlock()
	imageStreamNames := make([]string, len(b.imageStreamsToAutoRebuild))
	copy(imageStreamNames, b.imageStreamsToAutoRebuild)
	return imageStreamNames
}
