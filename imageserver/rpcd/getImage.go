package rpcd

import (
	"strings"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

type imageData struct {
	Name string
	*image.Image
}

func (t *srpcType) GetImage(conn *srpc.Conn,
	request imageserver.GetImageRequest,
	reply *imageserver.GetImageResponse) error {
	var response imageserver.GetImageResponse
	response.Image, response.InformationDatabaseURL = t.getImageNow(request)
	*reply = response
	if response.Image != nil || request.Timeout == 0 {
		return nil
	}
	// Image not found yet and willing to wait.
	addCh := t.imageDataBase.RegisterAddNotifier()
	defer func() {
		t.imageDataBase.UnregisterAddNotifier(addCh)
		select {
		case <-addCh:
		default:
		}
	}()
	timer := time.NewTimer(request.Timeout)
	for {
		select {
		case imageName := <-addCh:
			if imageName == request.ImageName {
				if !timer.Stop() {
					<-timer.C
				}
				response.Image, response.InformationDatabaseURL =
					t.getImageNow(request)
				*reply = response
				return nil
			}
		case <-timer.C:
			return nil
		}
	}
}

func (t *srpcType) getImageNow(
	request imageserver.GetImageRequest) (*image.Image, string) {
	originalImage := t.imageDataBase.GetImage(request.ImageName)
	if originalImage == nil {
		return nil, ""
	}
	img := *originalImage
	if request.IgnoreFilesystem {
		img.FileSystem = nil
	} else if request.IgnoreFilesystemIfExpiring &&
		!originalImage.ExpiresAt.IsZero() {
		img.FileSystem = nil
	}
	if t.informationDatabaseTemplate == nil {
		return &img, ""
	}
	data := imageData{
		Name:  request.ImageName,
		Image: &img,
	}
	builder := &strings.Builder{}
	err := t.informationDatabaseTemplate.Execute(builder, data)
	if err != nil {
		t.logger.Printf("%s: error executing template%s\n", err)
		return &img, ""
	} else {
		return &img, builder.String()
	}
}
