package filesystem

import (
	"fmt"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver"
)

func (objSrv *ObjectServer) adjustRefcounts(increment bool,
	iterator objectserver.ObjectsIterator) error {
	var count, size uint64
	var adjustedObjects []*objectType
	objSrv.rwLock.Lock()
	defer objSrv.rwLock.Unlock()
	startTime := time.Now()
	err := iterator.ForEachObject(func(hashVal hash.Hash) error {
		object := objSrv.objects[hashVal]
		if object == nil {
			return fmt.Errorf("unknown object: %x", hashVal)
		}
		if increment {
			if err := objSrv.incrementRefcount(object); err != nil {
				return err
			}
		} else {
			if err := objSrv.decrementRefcount(object); err != nil {
				return err
			}
		}
		size += object.size
		count++
		adjustedObjects = append(adjustedObjects, object)
		return nil
	})
	if err == nil {
		if increment {
			objSrv.Logger.Debugf(0,
				"Incremented refcounts, counted: %d (%s) in %s\n",
				count, format.FormatBytes(size),
				format.Duration(time.Since(startTime)))
		} else {
			objSrv.Logger.Debugf(0,
				"Decremented refcounts, counted: %d (%s) in %s\n",
				count, format.FormatBytes(size),
				format.Duration(time.Since(startTime)))
		}
		return nil
	}
	// Undo what was done so far.
	if increment {
		for _, object := range adjustedObjects {
			if err := objSrv.decrementRefcount(object); err != nil {
				panic(err)
			}
		}
	} else {
		for _, object := range adjustedObjects {
			if err := objSrv.incrementRefcount(object); err != nil {
				panic(err)
			}
		}
	}
	objSrv.Logger.Printf("Adjusted&reverted: %d (%s) in %s\n",
		count, format.FormatBytes(size),
		format.Duration(time.Since(startTime)))
	return err
}

// Add object to unreferenced list, at newest (front) position.
func (objSrv *ObjectServer) addUnreferenced(object *objectType) {
	object.olderUnreferenced = objSrv.newestUnreferenced
	if objSrv.oldestUnreferenced == nil {
		objSrv.oldestUnreferenced = object
	} else {
		objSrv.newestUnreferenced.newerUnreferenced = object
	}
	objSrv.newestUnreferenced = object
	objSrv.numUnreferenced++
	object.newerUnreferenced = nil
	objSrv.unreferencedBytes += object.size
}

// Decrement refcount and possibly add to list of unreferenced objects.
func (objSrv *ObjectServer) decrementRefcount(object *objectType) error {
	if object.refcount < 1 {
		return fmt.Errorf("cannot decrement zero refcount, object: %x",
			object.hash)
	}
	objSrv.duplicatedBytes -= object.size
	objSrv.numDuplicated--
	object.refcount--
	if object.refcount > 0 {
		return nil
	}
	objSrv.addUnreferenced(object)
	objSrv.numReferenced--
	objSrv.referencedBytes -= object.size
	return nil
}

// This must be called without the lock being held.
func (objSrv *ObjectServer) deleteOldestUnreferenced(lastPauseTime *time.Time) (
	uint64, error) {
	// Inject periodic pauses so that the write lockwatcher is not starved out.
	if time.Since(*lastPauseTime) > time.Second {
		time.Sleep(100 * time.Millisecond)
		*lastPauseTime = time.Now()
	}
	objSrv.rwLock.Lock()
	defer objSrv.rwLock.Unlock()
	object := objSrv.oldestUnreferenced
	if object == nil {
		return 0, fmt.Errorf("no more objects to delete")
	}
	if err := objSrv.deleteObject(object.hash, true); err != nil {
		return 0, err
	}
	return object.size, nil
}

// This must be called without the lock being held.
func (objSrv *ObjectServer) deleteUnreferenced(percentage uint8,
	bytesToDelete uint64) (uint64, uint64, error) {
	startTime := time.Now()
	var bytesDeleted, objectsDeleted uint64
	objSrv.rwLock.RLock()
	objectsToDelete := uint64(percentage) * objSrv.numUnreferenced / 100
	objSrv.rwLock.RUnlock()
	lastPauseTime := time.Now()
	for bytesDeleted < bytesToDelete || objectsDeleted < objectsToDelete {
		size, err := objSrv.deleteOldestUnreferenced(&lastPauseTime)
		if err != nil {
			return bytesDeleted, objectsDeleted, err
		}
		bytesDeleted += size
		objectsDeleted++
	}
	objSrv.Logger.Printf("Garbage collector deleted: %s in: %d objects in %s\n",
		format.FormatBytes(bytesDeleted), objectsDeleted,
		format.Duration(time.Since(startTime)))
	return bytesDeleted, objectsDeleted, nil
}

// Increment refcount and possibly remove from list of unreferenced objects.
func (objSrv *ObjectServer) incrementRefcount(object *objectType) error {
	if object.refcount < 1 {
		objSrv.numReferenced++
		objSrv.referencedBytes += object.size
		objSrv.removeUnreferenced(object)
	}
	objSrv.duplicatedBytes += object.size
	objSrv.numDuplicated++
	object.refcount++
	return nil
}

func (objSrv *ObjectServer) listUnreferenced() map[hash.Hash]uint64 {
	objSrv.rwLock.RLock()
	defer objSrv.rwLock.RUnlock()
	objects := make(map[hash.Hash]uint64, objSrv.numUnreferenced)
	for ob := objSrv.oldestUnreferenced; ob != nil; ob = ob.newerUnreferenced {
		objects[ob.hash] = ob.size
	}
	return objects
}

// Remove object from list if present, else do nothing.
func (objSrv *ObjectServer) removeUnreferenced(object *objectType) {
	var removed bool
	if object.olderUnreferenced == nil {
		if objSrv.oldestUnreferenced == object {
			objSrv.oldestUnreferenced = object.newerUnreferenced
			removed = true
		}
	} else {
		object.olderUnreferenced.newerUnreferenced = object.newerUnreferenced
		removed = true
	}
	if object.newerUnreferenced == nil {
		if objSrv.newestUnreferenced == object {
			objSrv.newestUnreferenced = object.olderUnreferenced
			removed = true
		}
	} else {
		object.newerUnreferenced.olderUnreferenced = object.olderUnreferenced
		removed = true
	}
	object.olderUnreferenced = nil
	if removed {
		objSrv.numUnreferenced--
		objSrv.unreferencedBytes -= object.size
	}
	object.newerUnreferenced = nil
}
