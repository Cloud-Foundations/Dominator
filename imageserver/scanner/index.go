package scanner

import (
	"sort"
	"strings"
	"sync"
)

type imageSortedIndex struct {
	mu    *sync.RWMutex
	index []string // sorted image names only.
}

func NewImageSortedIndex(mu *sync.RWMutex) *imageSortedIndex {
	if mu == nil {
		panic("imageSortedIndex requires a non-nil shared mutex")
	}
	return &imageSortedIndex{
		mu:    mu,
		index: make([]string, 0),
	}
}

// Check if imageSortedIndex implements ImageIndex.
var _ ImageIndex = &imageSortedIndex{}

// Add inserts a name while keeping slice sorted.
func (s *imageSortedIndex) Add(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := sort.SearchStrings(s.index, name)
	// avoid duplicates.
	if i < len(s.index) && s.index[i] == name {
		return
	}
	s.index = append(s.index, "")
	copy(s.index[i+1:], s.index[i:])
	s.index[i] = name
}

// Delete removes a name from the index.
func (s *imageSortedIndex) Delete(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := sort.SearchStrings(s.index, name)
	if i < len(s.index) && s.index[i] == name {
		s.index = append(s.index[:i], s.index[i+1:]...)
	}
}

// GetByPrefix returns all names under a directory-like prefix.
func (s *imageSortedIndex) GetByPrefix(prefix string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
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

// Get returns exact match.
func (s *imageSortedIndex) Get(name string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	i := sort.SearchStrings(s.index, name)
	if i < len(s.index) && s.index[i] == name {
		return s.index[i], true
	}
	return "", false
}
