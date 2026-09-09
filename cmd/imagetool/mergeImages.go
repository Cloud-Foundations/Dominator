package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/merge"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	objectclient "github.com/Cloud-Foundations/Dominator/lib/objectserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/triggers"
)

type namePrefixPair struct {
	name   string
	prefix string
}

func makeNamePrefixPairs(args []string) ([]namePrefixPair, error) {
	if len(args)%2 != 0 {
		return nil, fmt.Errorf("each image layer should have a prefix")
	}
	var pairs []namePrefixPair
	for index := range len(args) >> 1 {
		pairs = append(pairs, namePrefixPair{
			name:   args[index<<1],
			prefix: args[index<<1+1],
		})
	}
	return pairs, nil
}

func mergeImagesSubcommand(args []string, logger log.DebugLogger) error {
	pairs, err := makeNamePrefixPairs(args[1:])
	if err != nil {
		return err
	}
	imageSClient, objectClient := getClients()
	err = mergeImages(imageSClient, objectClient, args[0], pairs, logger)
	if err != nil {
		return fmt.Errorf("error merging images: %s", err)
	}
	return nil
}

func mergeImages(imageSClient *srpc.Client,
	objectClient *objectclient.ObjectClient, name string,
	pairs []namePrefixPair, logger log.DebugLogger) error {
	imageExists, err := client.CheckImage(imageSClient, name)
	if err != nil {
		return errors.New("error checking for image existence: " + err.Error())
	}
	if imageExists {
		return fmt.Errorf("image: %s exists", name)
	}
	startTime := time.Now()
	buildLog := &bytes.Buffer{}
	mergeableFS, err := merge.New(nil)
	if err != nil {
		return err
	}
	mergeableFilter := &filter.MergeableFilter{}
	mergeableTriggers := &triggers.MergeableTriggers{}
	for _, layer := range pairs {
		img, imageName, err := getTypedImageAndName(layer.name)
		if err != nil {
			return err
		}
		fmt.Fprintf(buildLog, "Merging image: %s at prefix: %s\n",
			imageName, layer.prefix)
		err = copyAnnotation(buildLog, objectClient, img.BuildLog)
		if err != nil {
			return err
		}
		options := &merge.Options{PrefixPath: layer.prefix}
		if err := mergeableFS.Merge(img.FileSystem, options); err != nil {
			return fmt.Errorf("error merging image: %s: %s", layer.name, err)
		}
		mergeableFilter.Merge(img.Filter)
		mergeableTriggers.Merge(img.Triggers)
		fmt.Fprintln(buildLog, strings.Repeat("=", 79))
	}
	fmt.Fprintf(buildLog, "Merged %d images in %s\n",
		len(pairs), format.Duration(time.Since(startTime)))
	buildLogHash, _, err := objectClient.AddObject(buildLog,
		uint64(buildLog.Len()), nil)
	if err != nil {
		return err
	}
	newImage := &image.Image{
		BuildLog:   &image.Annotation{Object: &buildLogHash},
		FileSystem: mergeableFS.GetFileSystem(),
		Filter:     mergeableFilter.ExportFilter(),
		Triggers:   mergeableTriggers.ExportTriggers(),
	}
	return addImage(imageSClient, name, newImage, logger)
}

func copyAnnotation(w io.Writer, objectClient *objectclient.ObjectClient,
	annotation *image.Annotation) error {
	if annotation == nil {
		return nil
	}
	if annotation.Object == nil {
		return nil
	}
	size, rc, err := objectClient.GetObject(*annotation.Object)
	if err != nil {
		return err
	}
	defer rc.Close()
	if size > 0 {
		fmt.Fprintln(w, "Build log follows:")
		_, err = io.Copy(w, rc)
		return err
	}
	return nil
}
