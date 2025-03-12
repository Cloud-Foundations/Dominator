package scanner

import (
	"os"
	"path"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/image"
)

func imageIsExpired(img *image.Image) bool {
	if !img.ExpiresAt.IsZero() && img.ExpiresAt.Sub(time.Now()) <= 0 {
		return true
	}
	return false
}

// This must not be called with the lock held.
func (imdb *ImageDataBase) expireImage(img *image.Image, name string) {
	if img.ExpiresAt.IsZero() {
		return
	}
	duration := img.ExpiresAt.Sub(time.Now())
	if duration > 0 {
		time.AfterFunc(duration, func() { imdb.expireImage(img, name) })
		return
	}
	imdb.Logger.Printf("Auto expiring (deleting) image: %s\n", name)
	pathname := path.Join(imdb.BaseDirectory, name)
	// Only rename file while lock is held, because removing can be slow.
	imdb.Lock()
	if err := os.Rename(pathname, pathname+"~"); err != nil {
		imdb.Logger.Println(err)
	}
	imdb.deleteImageAndUpdateUnreferencedObjectsList(name)
	imdb.Unlock()
	if err := os.Remove(pathname + "~"); err != nil {
		imdb.Logger.Println(err)
	}
}

// This may be called with the lock held.
func (imdb *ImageDataBase) scheduleExpiration(img *image.Image,
	name string) {
	if img.ExpiresAt.IsZero() {
		return
	}
	duration := img.ExpiresAt.Sub(time.Now())
	if duration <= 0 {
		return
	}
	time.AfterFunc(duration, func() { imdb.expireImage(img, name) })
	return
}
