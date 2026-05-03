package scanner

import (
	"slices"
	"strings"
)

// sortedImageIndex is an imageIndex backed by a sorted slice of image names.
type sortedImageIndex struct {
	index []string // sorted image names only.
}

func newImageSortedIndex() *sortedImageIndex {
	return &sortedImageIndex{index: make([]string, 0)}
}

var _ imageIndex = &sortedImageIndex{}

func (s *sortedImageIndex) Add(name string) {
	i, found := slices.BinarySearch(s.index, name)
	if found {
		return
	}
	s.index = slices.Insert(s.index, i, name)
}

func (s *sortedImageIndex) Delete(name string) {
	i, found := slices.BinarySearch(s.index, name)
	if !found {
		return
	}
	s.index = slices.Delete(s.index, i, i+1)
}

func (s *sortedImageIndex) GetByPrefix(prefix string) []string {
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	start, _ := slices.BinarySearch(s.index, prefix)
	if start == len(s.index) || !strings.HasPrefix(s.index[start], prefix) {
		return nil
	}
	relativeEnd, _ := slices.BinarySearchFunc(s.index[start:], prefix,
		func(e, t string) int {
			if strings.HasPrefix(e, t) {
				// Move the search outside of prefix block.
				return -1
			}
			return strings.Compare(e, t)
		},
	)
	end := start + relativeEnd
	return slices.Clone(s.index[start:end])
}

func (s *sortedImageIndex) Get(name string) (string, bool) {
	i, found := slices.BinarySearch(s.index, name)
	if !found {
		return "", false
	}
	return s.index[i], true
}
