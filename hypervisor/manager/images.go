package manager

import (
	"fmt"
	"time"

	imclient "github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	// "github.com/Cloud-Foundations/Dominator/lib/objectcache"
	// "github.com/Cloud-Foundations/Dominator/lib/objectserver"
	// objclient "github.com/Cloud-Foundations/Dominator/lib/objectserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

type imageDownloadData struct {
	connectTime time.Duration
	fetchTime   time.Duration
	image       *image.Image
	imageName   string
	rebuildTime time.Duration
	srpcClient  srpc.ClientI
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
		img, err := imclient.GetImage(client, imageName)
		if err != nil {
			return nil, err
		}
		loadedTime := time.Now()
		if err := img.FileSystem.RebuildInodePointers(); err != nil {
			return nil, err
		}
		rebuiltTime := time.Now()
		doClose = false
		return &imageDownloadData{
			connectTime: connectedTime.Sub(startTime),
			fetchTime:   loadedTime.Sub(connectedTime),
			image:       img,
			imageName:   imageName,
			rebuildTime: rebuiltTime.Sub(loadedTime),
			srpcClient:  client,
		}, nil
	}
	img, err := imclient.GetImageWithTimeout(client, searchName, imageTimeout)
	if err != nil {
		return nil, err
	}
	if img == nil {
		return nil, errors.New("timeout getting image")
	}
	loadedTime := time.Now()
	if err := img.FileSystem.RebuildInodePointers(); err != nil {
		return nil, err
	}
	rebuiltTime := time.Now()
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
	hashes := make([]hash.Hash, 0, len(dd.image.FileSystem.InodeTable))
	hashMap := make(map[hash.Hash]struct{}, len(dd.image.FileSystem.InodeTable))
	var imageBytes uint64
	for _, inode := range dd.image.FileSystem.InodeTable {
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

func (m *Manager) getImage(searchName string, imageTimeout time.Duration,
	logger log.DebugLogger) (srpc.ClientI, *image.Image, string, error) {
	dd, err := m.downloadImage(searchName, imageTimeout)
	if err != nil {
		return nil, nil, "", err
	}
	logger.Printf(
		"loaded: %s in %s, built inode pointers in: %s, connected in: %s\n",
		dd.imageName,
		format.Duration(dd.fetchTime),
		format.Duration(dd.rebuildTime),
		format.Duration(dd.connectTime))
	return dd.srpcClient, dd.image, dd.imageName, nil
}
