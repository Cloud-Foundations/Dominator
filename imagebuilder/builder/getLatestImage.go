package builder

import (
	"fmt"
	"io"
	"time"

	imageclient "github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	image_proto "github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func getLatestImage(client srpc.ClientI, imageStream, buildCommitId string,
	buildLog io.Writer) (string, *image.Image, error) {
	imageName, err := imageclient.FindLatestImageReq(client,
		image_proto.FindLatestImageRequest{
			BuildCommitId: buildCommitId,
			DirectoryName: imageStream,
		})
	if err != nil {
		return "", nil, err
	}
	if imageName == "" {
		return "", nil, nil
	}
	if img, err := getImage(client, imageName, buildLog); err != nil {
		return "", nil, err
	} else {
		return imageName, img, nil
	}
}

func getImage(client srpc.ClientI, imageName string, buildLog io.Writer) (
	*image.Image, error) {
	startTime := time.Now()
	if img, err := imageclient.GetImage(client, imageName); err != nil {
		return nil, err
	} else {
		startRebuildTime := time.Now()
		img.FileSystem.RebuildInodePointers()
		finishedTime := time.Now()
		fmt.Fprintf(buildLog, "Downloaded %s in %s, rebuilt pointers in %s\n",
			imageName,
			format.Duration(startRebuildTime.Sub(startTime)),
			format.Duration(finishedTime.Sub(startRebuildTime)))
		return img, nil
	}
}
