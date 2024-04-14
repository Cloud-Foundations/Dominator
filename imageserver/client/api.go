package client

import (
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func AddImage(client srpc.ClientI, name string, img *image.Image) error {
	return addImage(client, name, img)
}

func AddImageTrusted(client srpc.ClientI, name string, img *image.Image) error {
	return addImageTrusted(client, name, img)
}

func ChangeImageExpiration(client srpc.ClientI, name string,
	expiresAt time.Time) error {
	return changeImageExpiration(client, name, expiresAt)
}

func CheckDirectory(client srpc.ClientI, name string) (bool, error) {
	return checkDirectory(client, name)
}

func CheckImage(client srpc.ClientI, name string) (bool, error) {
	return checkImage(client, name)
}

func ChownDirectory(client srpc.ClientI, dirname, ownerGroup string) error {
	return chownDirectory(client, dirname, ownerGroup)
}

func DeleteImage(client srpc.ClientI, name string) error {
	return deleteImage(client, name)
}

func DeleteUnreferencedObjects(client srpc.ClientI, percentage uint8,
	bytes uint64) error {
	return deleteUnreferencedObjects(client, percentage, bytes)
}

func FindLatestImage(client srpc.ClientI, dirname string,
	ignoreExpiring bool) (string, error) {
	return findLatestImage(client, proto.FindLatestImageRequest{
		DirectoryName:        dirname,
		IgnoreExpiringImages: ignoreExpiring,
	})
}

func FindLatestImageReq(client srpc.ClientI,
	request proto.FindLatestImageRequest) (string, error) {
	return findLatestImage(client, request)
}

func GetImage(client srpc.ClientI, name string) (*image.Image, error) {
	return getImage(client, name, 0)
}

func GetImageExpiration(client srpc.ClientI, name string) (time.Time, error) {
	return getImageExpiration(client, name)
}

func GetReplicationMaster(client srpc.ClientI) (string, error) {
	return getReplicationMaster(client)
}

func GetImageWithTimeout(client srpc.ClientI, name string,
	timeout time.Duration) (*image.Image, error) {
	return getImage(client, name, timeout)
}

func ListDirectories(client srpc.ClientI) ([]image.Directory, error) {
	return listDirectories(client)
}

func ListImages(client srpc.ClientI) ([]string, error) {
	return listImages(client)
}

func ListSelectedImages(client srpc.ClientI,
	request proto.ListSelectedImagesRequest) ([]string, error) {
	return listSelectedImages(client, request)
}

func ListUnreferencedObjects(client srpc.ClientI) (
	map[hash.Hash]uint64, error) {
	return listUnreferencedObjects(client)
}

func MakeDirectory(client srpc.ClientI, dirname string) error {
	return makeDirectory(client, dirname)
}
