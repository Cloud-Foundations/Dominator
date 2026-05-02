package scanner

import (
	"sort"
	"strings"
)

// imageSortedIndex is an ImageIndex backed by a sorted slice of image names.
type imageSortedIndex struct {
	index []string // sorted image names only.
}

func NewImageSortedIndex() *imageSortedIndex {
	return &imageSortedIndex{index: make([]string, 0)}
}

var _ ImageIndex = &imageSortedIndex{}

func (s *imageSortedIndex) Add(name string) {
	i := sort.SearchStrings(s.index, name)
	if i < len(s.index) && s.index[i] == name {
		return
	}
	s.index = append(s.index, "")
	copy(s.index[i+1:], s.index[i:])
	s.index[i] = name
}

func (s *imageSortedIndex) Delete(name string) {
	i := sort.SearchStrings(s.index, name)
	if i < len(s.index) && s.index[i] == name {
		s.index = append(s.index[:i], s.index[i+1:]...)
	}
}

func (s *imageSortedIndex) GetByPrefix(prefix string) []string {
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	start := sort.SearchStrings(s.index, prefix)
	end := sort.Search(len(s.index), func(i int) bool {
		return !strings.HasPrefix(s.index[i], prefix)
	})
	out := make([]string, end-start)
	copy(out, s.index[start:end])
	return out
}

func (s *imageSortedIndex) Get(name string) (string, bool) {
	i := sort.SearchStrings(s.index, name)
	if i < len(s.index) && s.index[i] == name {
		return s.index[i], true
	}
	return "", false
}
