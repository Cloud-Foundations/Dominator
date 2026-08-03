package fstree

import (
	"io"

	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

const (
	TypeBlob    = 0
	TypeTree    = 1
	TypeSymlink = 2
)

type Getter interface {
	GetBlobData(hashVal hash.Hash) ([]byte, error)
	GetBlobReader(hashVal hash.Hash) (io.ReadCloser, uint64, error)
	GetTree(hashVal hash.Hash) (*Tree, error)
	GetTreeReader(hashVal hash.Hash) (io.ReadCloser, uint64, error)
}

type GetterParams struct {
	BaseUrl     string
	IoSemaphore chan struct{}
	Logger      log.DebugLogger
}

type Tree struct {
	Entries []TreeEntry
}

type TreeEntry struct {
	Filename    string
	GroupId     uint32
	Hash        hash.Hash
	Permissions uint32
	Size        uint64
	Type        uint32
	UserId      uint32
}

type TreeWalker func(getter Getter, dirname string, entry *TreeEntry) error

type WalkParams struct {
	Function TreeWalker
	Getter   Getter
	Logger   log.DebugLogger
	TreeUrl  string
}

func DecodeTree(r io.Reader) (*Tree, error) {
	return decodeTree(r)
}

func NewGetter(params GetterParams) (Getter, error) {
	return newGetter(params)
}

func SplitTreeUrl(treeUrl string) (baseUrl, hash string, err error) {
	return splitTreeUrl(treeUrl)
}

func WalkTree(params WalkParams) error {
	return walkTree(params)
}
