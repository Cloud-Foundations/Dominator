package objectserver

import (
	"io"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/hash"
)

type FullObjectServer interface {
	DeleteObject(hashVal hash.Hash) error
	ObjectServer
	ObjectsRefcounter
	LastMutationTime() time.Time
	ListObjectSizes() map[hash.Hash]uint64
	ListObjects() []hash.Hash
	NumObjects() uint64
}

type AddCallback func(hashVal hash.Hash, length uint64, isNew bool)

type AddCallbackSetter interface {
	SetAddCallback(callback AddCallback)
}

type GarbageCollector func(bytesToDelete uint64) (
	bytesDeleted uint64, err error)

type GarbageCollectorSetter interface {
	SetGarbageCollector(gc GarbageCollector)
}

type ObjectLinker interface {
	LinkObject(filename string, hashVal hash.Hash) (bool, error)
}

type ObjectGetter interface {
	GetObject(hashVal hash.Hash) (uint64, io.ReadCloser, error)
}

type ObjectsChecker interface {
	CheckObjects(hashes []hash.Hash) ([]uint64, error)
}

type ObjectsGetter interface {
	GetObjects(hashes []hash.Hash) (ObjectsReader, error)
}

type ObjectsIterator interface {
	ForEachObject(objectFunc func(hash.Hash) error) error
}

type ObjectsRefcounter interface {
	AdjustRefcounts(bool, ObjectsIterator) error
	DeleteUnreferenced(percentage uint8, bytes uint64) (uint64, uint64, error)
	ListUnreferenced() map[hash.Hash]uint64
}

type FullObjectsReader interface {
	ObjectsReader
	ObjectSizes() []uint64
}

type ObjectsReader interface {
	Close() error
	NextObject() (uint64, io.ReadCloser, error)
}

type ObjectServer interface {
	AddObject(reader io.Reader, length uint64, expectedHash *hash.Hash) (
		hash.Hash, bool, error)
	ObjectGetter
	ObjectsChecker
	ObjectsGetter
}

type StashingObjectServer interface {
	CommitObject(hash.Hash) error
	DeleteStashedObject(hashVal hash.Hash) error
	ObjectServer
	StashOrVerifyObject(io.Reader, uint64, *hash.Hash) (
		hash.Hash, []byte, error)
}

func CopyObject(filename string, objectsGetter ObjectsGetter,
	hashVal hash.Hash) error {
	return copyObject(filename, objectsGetter, hashVal)
}

func GetObject(objSrv ObjectsGetter, hashVal hash.Hash) (
	uint64, io.ReadCloser, error) {
	return getObject(objSrv, hashVal)
}

func LinkObject(filename string, objectsGetter ObjectsGetter,
	hashVal hash.Hash) (bool, error) {
	return linkObject(filename, objectsGetter, hashVal)
}
