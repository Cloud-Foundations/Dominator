package scanner

import (
	"os"
	"path"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/image"
)

func imageIsExpired(image *image.Image) bool {
	if !image.ExpiresAt.IsZero() && image.ExpiresAt.Sub(time.Now()) <= 0 {
		return true
	}
	return false
}

// This must not be called with the lock held.
func (imdb *ImageDataBase) expireImage(image *image.Image, name string) {
	if image.ExpiresAt.IsZero() {
		return
	}
	duration := image.ExpiresAt.Sub(time.Now())
	if duration > 0 {
		time.AfterFunc(duration, func() { imdb.expireImage(image, name) })
		return
	}
	imdb.Lock()
	defer imdb.Unlock()
	imdb.Logger.Printf("Auto expiring (deleting) image: %s\n", name)
	if err := os.Remove(path.Join(imdb.BaseDirectory, name)); err != nil {
		imdb.Logger.Println(err)
	}
	imdb.deleteImageAndUpdateUnreferencedObjectsList(name)
}

// This may be called with the lock held.
func (imdb *ImageDataBase) scheduleExpiration(image *image.Image,
	name string) {
	if image.ExpiresAt.IsZero() {
		return
	}
	duration := image.ExpiresAt.Sub(time.Now())
	if duration <= 0 {
		return
	}
	time.AfterFunc(duration, func() { imdb.expireImage(image, name) })
	return
}
