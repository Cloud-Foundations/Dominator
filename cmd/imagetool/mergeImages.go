package main

import (
	"errors"
	"fmt"

	"github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/merge"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/log"
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
	imageSClient, _ := getClients()
	if err := mergeImages(imageSClient, args[0], pairs, logger); err != nil {
		return fmt.Errorf("error merging images: %s", err)
	}
	return nil
}

func mergeImages(imageSClient *srpc.Client, name string, pairs []namePrefixPair,
	logger log.DebugLogger) error {
	imageExists, err := client.CheckImage(imageSClient, name)
	if err != nil {
		return errors.New("error checking for image existence: " + err.Error())
	}
	if imageExists {
		return fmt.Errorf("image: %s exists", name)
	}
	mergeableFS, err := merge.New(nil)
	if err != nil {
		return err
	}
	mergeableFilter := &filter.MergeableFilter{}
	mergeableTriggers := &triggers.MergeableTriggers{}
	for _, layer := range pairs {
		img, err := getTypedImage(layer.name)
		if err != nil {
			return err
		}
		options := &merge.Options{PrefixPath: layer.prefix}
		if err := mergeableFS.Merge(img.FileSystem, options); err != nil {
			return fmt.Errorf("error merging image: %s: %s", layer.name, err)
		}
		mergeableFilter.Merge(img.Filter)
		mergeableTriggers.Merge(img.Triggers)
	}
	newImage := &image.Image{
		FileSystem: mergeableFS.GetFileSystem(),
		Filter:     mergeableFilter.ExportFilter(),
		Triggers:   mergeableTriggers.ExportTriggers(),
	}
	return addImage(imageSClient, name, newImage, logger)
}
