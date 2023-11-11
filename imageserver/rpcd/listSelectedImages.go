package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func (t *srpcType) ListSelectedImages(conn *srpc.Conn) error {
	var request proto.ListSelectedImagesRequest
	if err := conn.Decode(&request); err != nil {
		return err
	}
	for _, name := range t.imageDataBase.ListSelectedImages(request) {
		if _, err := conn.WriteString(name + "\n"); err != nil {
			return err
		}
	}
	_, err := conn.WriteString("\n")
	return err
}
