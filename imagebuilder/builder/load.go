package builder

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/imagebuilder/logarchiver"
	"github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/configwatch"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	libjson "github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
	"github.com/Cloud-Foundations/Dominator/lib/triggers"
	"github.com/Cloud-Foundations/Dominator/lib/url/urlutil"
)

func getNamespace() (string, error) {
	pathname := fmt.Sprintf("/proc/%d/ns/mnt", syscall.Gettid())
	namespace, err := os.Readlink(pathname)
	if err != nil {
		return "", fmt.Errorf("error discovering namespace: %s", err)
	}
	return namespace, nil
}

func imageStreamsDecoder(reader io.Reader) (interface{}, error) {
	return imageStreamsRealDecoder(reader)
}

func imageStreamsRealDecoder(reader io.Reader) (
	*imageStreamsConfigurationType, error) {
	var config imageStreamsConfigurationType
	decoder := json.NewDecoder(bufio.NewReader(reader))
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}
	for _, stream := range config.Streams {
		stream.builderUsers = stringutil.ConvertListToMap(stream.BuilderUsers,
			false)
	}
	return &config, nil
}

func load(options BuilderOptions, params BuilderParams) (*Builder, error) {
	ctimeResolution, err := getCtimeResolution()
	if err != nil {
		return nil, err
	}
	if params.BuildLogArchiver == nil {
		params.BuildLogArchiver = logarchiver.NewNullLogger()
	}
	params.Logger.Printf("Inode Ctime resolution: %s\n",
		format.Duration(ctimeResolution))
	initialNamespace, err := getNamespace()
	if err != nil {
		return nil, err
	}
	params.Logger.Printf("Initial namespace: %s\n", initialNamespace)
	err = syscall.Mount("none", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, "")
	if err != nil {
		return nil, fmt.Errorf("error making mounts private: %s", err)
	}
	masterConfiguration, err := loadMasterConfiguration(
		options.ConfigurationURL)
	if err != nil {
		return nil, fmt.Errorf("error getting master configuration: %s", err)
	}
	if len(masterConfiguration.BootstrapStreams) < 1 {
		params.Logger.Println(
			"No bootstrap streams configured: some operations degraded")
	}
	imageStreamsToAutoRebuild := make([]string, 0)
	for name := range masterConfiguration.BootstrapStreams {
		imageStreamsToAutoRebuild = append(imageStreamsToAutoRebuild, name)
	}
	sort.Strings(imageStreamsToAutoRebuild)
	for _, name := range masterConfiguration.ImageStreamsToAutoRebuild {
		imageStreamsToAutoRebuild = append(imageStreamsToAutoRebuild, name)
	}
	var variables map[string]string
	if options.VariablesFile != "" {
		err := libjson.ReadFromFile(options.VariablesFile, &variables)
		if err != nil {
			return nil, err
		}
	}
	if variables == nil {
		variables = make(map[string]string)
	}
	generateDependencyTrigger := make(chan chan<- struct{}, 1)
	streamsLoadedChannel := make(chan struct{})
	b := &Builder{
		buildLogArchiver:          params.BuildLogArchiver,
		bindMounts:                masterConfiguration.BindMounts,
		generateDependencyTrigger: generateDependencyTrigger,
		stateDir:                  options.StateDirectory,
		imageServerAddress:        options.ImageServerAddress,
		logger:                    params.Logger,
		imageStreamsUrl:           masterConfiguration.ImageStreamsUrl,
		initialNamespace:          initialNamespace,
		streamsLoadedChannel:      streamsLoadedChannel,
		bootstrapStreams:          masterConfiguration.BootstrapStreams,
		imageStreamsToAutoRebuild: imageStreamsToAutoRebuild,
		slaveDriver:               params.SlaveDriver,
		currentBuildInfos:         make(map[string]*currentBuildInfo),
		lastBuildResults:          make(map[string]buildResultType),
		packagerTypes:             masterConfiguration.PackagerTypes,
		relationshipsQuickLinks:   masterConfiguration.RelationshipsQuickLinks,
		variables:                 variables,
	}
	for name, stream := range b.bootstrapStreams {
		stream.builder = b
		stream.name = name
	}
	imageStreamsConfigChannel, err := configwatch.WatchWithCache(
		masterConfiguration.ImageStreamsUrl,
		time.Second*time.Duration(
			masterConfiguration.ImageStreamsCheckInterval), imageStreamsDecoder,
		filepath.Join(options.StateDirectory, "image-streams.json"),
		time.Second*5, params.Logger)
	if err != nil {
		return nil, err
	}
	go b.dependencyGeneratorLoop(generateDependencyTrigger)
	go b.watchConfigLoop(imageStreamsConfigChannel, streamsLoadedChannel)
	go b.rebuildImages(options.ImageRebuildInterval)
	return b, nil
}

func loadImageStreams(url string) (*imageStreamsConfigurationType, error) {
	if url == "" {
		return &imageStreamsConfigurationType{}, nil
	}
	file, err := urlutil.Open(url)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	configuration, err := imageStreamsRealDecoder(file)
	if err != nil {
		return nil, fmt.Errorf("error decoding image streams from: %s: %s",
			url, err)
	}
	return configuration, nil
}

func loadMasterConfiguration(url string) (*masterConfigurationType, error) {
	file, err := urlutil.Open(url)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var configuration masterConfigurationType
	decoder := json.NewDecoder(bufio.NewReader(file))
	if err := decoder.Decode(&configuration); err != nil {
		return nil, fmt.Errorf("error reading configuration from: %s: %s",
			url, err)
	}
	for _, stream := range configuration.BootstrapStreams {
		if _, ok := configuration.PackagerTypes[stream.PackagerType]; !ok {
			return nil, fmt.Errorf("packager type: \"%s\" unknown",
				stream.PackagerType)
		}
		if stream.Filter != nil {
			if err := stream.Filter.Compile(); err != nil {
				return nil, err
			}
		}
		if stream.ImageFilterUrl != "" {
			filterFile, err := urlutil.Open(stream.ImageFilterUrl)
			if err != nil {
				return nil, err
			}
			defer filterFile.Close()
			stream.imageFilter, err = filter.Read(filterFile)
			if err != nil {
				return nil, err
			}
		}
		if stream.ImageTriggersUrl != "" {
			triggersFile, err := urlutil.Open(stream.ImageTriggersUrl)
			if err != nil {
				return nil, err
			}
			defer triggersFile.Close()
			stream.imageTriggers, err = triggers.Read(triggersFile)
			if err != nil {
				return nil, err
			}
		}
	}
	return &configuration, nil
}

func (b *Builder) delayMakeRequiredDirectories(abortNotifier <-chan struct{}) {
	timer := time.NewTimer(time.Second * 5)
	select {
	case <-abortNotifier:
		if !timer.Stop() {
			<-timer.C
		}
	case <-timer.C:
		b.makeRequiredDirectories()
	}
}

func (b *Builder) makeRequiredDirectories() error {
	imageServer, err := srpc.DialHTTP("tcp", b.imageServerAddress, 0)
	if err != nil {
		b.logger.Printf("%s: %s\n", b.imageServerAddress, err)
		return nil
	}
	defer imageServer.Close()
	directoryList, err := client.ListDirectories(imageServer)
	if err != nil {
		b.logger.Println(err)
		return nil
	}
	directories := make(map[string]struct{}, len(directoryList))
	for _, directory := range directoryList {
		directories[directory.Name] = struct{}{}
	}
	streamNames := b.listAllStreamNames()
	for _, streamName := range streamNames {
		if _, ok := directories[streamName]; ok {
			continue
		}
		pathComponents := strings.Split(streamName, "/")
		for index := range pathComponents {
			partPath := strings.Join(pathComponents[0:index+1], "/")
			if _, ok := directories[partPath]; ok {
				continue
			}
			if err := client.MakeDirectory(imageServer, partPath); err != nil {
				return err
			}
			b.logger.Printf("Created missing directory: %s\n", partPath)
			directories[partPath] = struct{}{}
		}
	}
	return nil
}

func (b *Builder) reloadNormalStreamsConfiguration() error {
	imageStreamsConfiguration, err := loadImageStreams(b.imageStreamsUrl)
	if err != nil {
		return err
	}
	b.logger.Println("Reloaded streams streams configuration")
	return b.updateImageStreams(imageStreamsConfiguration)
}

func (b *Builder) updateImageStreams(
	imageStreamsConfiguration *imageStreamsConfigurationType) error {
	for name, stream := range imageStreamsConfiguration.Streams {
		stream.builder = b
		stream.name = name
	}
	b.streamsLock.Lock()
	b.imageStreams = imageStreamsConfiguration.Streams
	b.streamsLock.Unlock()
	b.triggerDependencyDataGeneration()
	return b.makeRequiredDirectories()
}

func (b *Builder) waitForStreamsLoaded(timeout time.Duration) error {
	if timeout < 0 {
		timeout = time.Hour
	} else if timeout < time.Second {
		timeout = time.Second
	}
	timer := time.NewTimer(timeout)
	select {
	case <-b.streamsLoadedChannel:
		if !timer.Stop() {
			<-timer.C
		}
		return nil
	case <-timer.C:
		return fmt.Errorf("timed out waiting for streams list to load")
	}
}

func (b *Builder) watchConfigLoop(configChannel <-chan interface{},
	streamsLoadedChannel chan<- struct{}) {
	firstLoadNotifier := make(chan struct{})
	go b.delayMakeRequiredDirectories(firstLoadNotifier)
	for rawConfig := range configChannel {
		imageStreamsConfig, ok := rawConfig.(*imageStreamsConfigurationType)
		if !ok {
			b.logger.Printf("received unknown type over config channel")
			continue
		}
		if firstLoadNotifier != nil {
			firstLoadNotifier <- struct{}{}
			close(firstLoadNotifier)
			firstLoadNotifier = nil
		}
		if streamsLoadedChannel != nil {
			close(streamsLoadedChannel)
			streamsLoadedChannel = nil
		}
		b.logger.Println("received new image streams configuration")
		b.updateImageStreams(imageStreamsConfig)
	}
}
