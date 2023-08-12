package client

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func listSelectedImages(client *srpc.Client,
	request proto.ListSelectedImagesRequest) ([]string, error) {
	conn, err := client.Call("ImageServer.ListSelectedImages")
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if err := conn.Encode(request); err != nil {
		return nil, err
	}
	if err := conn.Flush(); err != nil {
		return nil, err
	}
	images := make([]string, 0)
	for {
		line, err := conn.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = line[:len(line)-1]
		if line == "" {
			break
		}
		images = append(images, line)
	}
	return images, nil
}
