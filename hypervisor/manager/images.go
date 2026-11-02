package manager

import (
	"fmt"
	"sync"
	"time"

	imclient "github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/list"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

type imageCacheType struct {
	mutex       sync.Mutex
	list        *list.UniqueList[string] // K: name. Front most recently used.
	nameToImage map[string]*imageType    // Key: name.
}

type imageType struct {
	image         *image.Image
	usageEstimate uint64
	mutex         sync.Mutex // Protect everything below.
	builtExtras   bool
}

type imageDownloadData struct {
	connectTime time.Duration
	fetchTime   time.Duration // 0: was cached.
	image       *imageType
	imageName   string
	rebuildTime time.Duration
	srpcClient  srpc.ClientI
}

func (img *imageType) buildExtraData() {
	// TODO(rgooch): Consider moving this into the lib/filesystem package.
	img.mutex.Lock()
	defer img.mutex.Unlock()
	if img.builtExtras {
		return
	}
	img.image.FileSystem.InodeToFilenamesTable()
	img.image.FileSystem.FilenameToInodeTable()
	img.image.FileSystem.HashToInodesTable()
	img.image.FileSystem.BuildEntryMap()
	img.builtExtras = true
}

func (m *Manager) cacheImage(imageName string,
	rawImage *image.Image) *imageType {
	m.imageCache.mutex.Lock()
	locked := true
	defer func() {
		if locked {
			m.imageCache.mutex.Unlock()
		}
	}()
	img, ok := m.imageCache.nameToImage[imageName]
	if ok {
		m.imageCache.list.PushFront(imageName)
		return img // Return existing, drop new.
	}
	img = &imageType{
		image:         rawImage,
		usageEstimate: rawImage.FileSystem.EstimateUsage(0),
	}
	m.imageCache.nameToImage[imageName] = img
	m.imageCache.list.PushFront(imageName)
	if m.imageCache.list.Length() < m.ImageCacheSize {
		return img
	}
	oldestEntry := m.imageCache.list.Back()
	delete(m.imageCache.nameToImage, oldestEntry.Value())
	oldestEntry.Remove()
	m.imageCache.mutex.Unlock()
	locked = false
	m.Logger.Printf("Dropped: %s from the image cache\n", imageName)
	return img
}

// downloadImage will download an image and return the image and information
// about the download.
func (m *Manager) downloadImage(searchName string, imageTimeout time.Duration) (
	*imageDownloadData, error) {
	// TODO(rgooch): Consider ways to re-use the connection while handling
	//               multiple concurrent users and/or use something like
	//               dom/images.
	startTime := time.Now()
	client, err := srpc.DialHTTP("tcp", m.ImageServerAddress, 0)
	if err != nil {
		return nil, fmt.Errorf("error connecting to image server: %s: %s",
			m.ImageServerAddress, err)
	}
	doClose := true
	defer func() {
		if doClose {
			client.Close()
		}
	}()
	connectedTime := time.Now()
	if isDir, err := imclient.CheckDirectory(client, searchName); err != nil {
		return nil, err
	} else if isDir {
		imageName, err := imclient.FindLatestImage(client, searchName, false)
		if err != nil {
			return nil, err
		}
		if imageName == "" {
			return nil, errors.New("no images in directory: " + searchName)
		}
		if img := m.getCachedImage(imageName); img != nil {
			doClose = false
			return &imageDownloadData{
				connectTime: connectedTime.Sub(startTime),
				image:       img,
				imageName:   imageName,
				srpcClient:  client,
			}, nil
		}
		rawImage, err := imclient.GetImage(client, imageName)
		if err != nil {
			return nil, err
		}
		loadedTime := time.Now()
		if err := rawImage.FileSystem.RebuildInodePointers(); err != nil {
			return nil, err
		}
		rebuiltTime := time.Now()
		doClose = false
		img := m.cacheImage(imageName, rawImage)
		return &imageDownloadData{
			connectTime: connectedTime.Sub(startTime),
			fetchTime:   loadedTime.Sub(connectedTime),
			image:       img,
			imageName:   imageName,
			rebuildTime: rebuiltTime.Sub(loadedTime),
			srpcClient:  client,
		}, nil
	}
	if img := m.getCachedImage(searchName); img != nil {
		doClose = false
		return &imageDownloadData{
			connectTime: connectedTime.Sub(startTime),
			image:       img,
			imageName:   searchName,
			srpcClient:  client,
		}, nil
	}
	rawImage, err := imclient.GetImageWithTimeout(client, searchName,
		imageTimeout)
	if err != nil {
		return nil, err
	}
	if rawImage == nil {
		return nil, errors.New("timeout getting image")
	}
	loadedTime := time.Now()
	if err := rawImage.FileSystem.RebuildInodePointers(); err != nil {
		return nil, err
	}
	rebuiltTime := time.Now()
	img := m.cacheImage(searchName, rawImage)
	doClose = false
	return &imageDownloadData{
		connectTime: connectedTime.Sub(startTime),
		fetchTime:   loadedTime.Sub(connectedTime),
		image:       img,
		imageName:   searchName,
		rebuildTime: rebuiltTime.Sub(loadedTime),
		srpcClient:  client,
	}, nil
}

// fetchImage will download an image and fetch objects into the cache.
func (m *Manager) fetchImage(searchName string, imageTimeout time.Duration) (
	proto.FetchImageResponse, error) {
	if m.objectCache == nil {
		return proto.FetchImageResponse{},
			errors.New("no object cache configured")
	}
	dd, err := m.downloadImage(searchName, imageTimeout)
	if err != nil {
		return proto.FetchImageResponse{}, err
	}
	defer dd.srpcClient.Close()
	fs := dd.image.image.FileSystem
	hashes := make([]hash.Hash, 0, len(fs.InodeTable))
	hashMap := make(map[hash.Hash]struct{}, len(fs.InodeTable))
	var imageBytes uint64
	for _, inode := range fs.InodeTable {
		if inode, ok := inode.(*filesystem.RegularInode); ok {
			if inode.Size > 0 {
				if _, exists := hashMap[inode.Hash]; !exists {
					hashes = append(hashes, inode.Hash)
					hashMap[inode.Hash] = struct{}{}
					imageBytes += inode.Size
				}
			}
		}
	}
	startTime := time.Now()
	stats, err := m.objectCache.FetchObjectsWithStats(hashes)
	if err != nil {
		return proto.FetchImageResponse{}, err
	}
	fetchedTime := time.Now()
	if dd.fetchTime > 0 {
		m.Logger.Printf(
			"Fetched: %s in %s, rebuilt inodes in: %s, downloaded %d/%d objects, %s/%s in: %s\n",
			dd.imageName,
			format.Duration(dd.fetchTime),
			format.Duration(dd.rebuildTime),
			stats.DownloadedObjects,
			len(hashes),
			format.FormatBytes(stats.DownloadedBytes),
			format.FormatBytes(imageBytes),
			format.Duration(fetchedTime.Sub(startTime)))
	} else {
		m.Logger.Printf(
			"Downloaded %d/%d objects, %s/%s in: %s\n",
			stats.DownloadedObjects,
			len(hashes),
			format.FormatBytes(stats.DownloadedBytes),
			format.FormatBytes(imageBytes),
			format.Duration(fetchedTime.Sub(startTime)))
	}
	return proto.FetchImageResponse{
		DownloadTime:      fetchedTime.Sub(startTime),
		DownloadedBytes:   stats.DownloadedBytes,
		DownloadedObjects: stats.DownloadedObjects,
		ImageBytes:        imageBytes,
		ImageFetchTime:    dd.fetchTime,
		ImageName:         dd.imageName,
		ImageObjects:      uint(len(hashes)),
		ImageRebuildTime:  dd.rebuildTime,
	}, nil
}

func (m *Manager) getCachedImage(imageName string) *imageType {
	m.imageCache.mutex.Lock()
	defer m.imageCache.mutex.Unlock()
	img, ok := m.imageCache.nameToImage[imageName]
	if !ok {
		return nil
	}
	rawImage := img.image
	if !rawImage.ExpiresAt.IsZero() && time.Since(rawImage.ExpiresAt) > 0 {
		delete(m.imageCache.nameToImage, imageName)
		m.imageCache.list.Remove(imageName)
		return nil
	}
	m.imageCache.list.PushFront(imageName)
	return img
}

func (m *Manager) getImage(searchName string, imageTimeout time.Duration,
	logger log.DebugLogger) (srpc.ClientI, *imageType, string, error) {
	dd, err := m.downloadImage(searchName, imageTimeout)
	if err != nil {
		return nil, nil, "", err
	}
	if dd.fetchTime > 0 {
		logger.Printf(
			"loaded: %s in %s, built inode pointers in: %s, connected in: %s\n",
			dd.imageName,
			format.Duration(dd.fetchTime),
			format.Duration(dd.rebuildTime),
			format.Duration(dd.connectTime))
	} else {
		logger.Printf("connected in: %s\n", format.Duration(dd.connectTime))
	}
	return dd.srpcClient, dd.image, dd.imageName, nil
}
