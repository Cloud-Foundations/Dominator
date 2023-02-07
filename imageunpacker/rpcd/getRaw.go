package rpcd

import (
	"io"

	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/imageunpacker"
)

func (t *srpcType) GetRaw(conn *srpc.Conn) error {
	var request proto.GetRawRequest
	if err := conn.Decode(&request); err != nil {
		return err
	}
	var reply proto.GetRawResponse
	reader, length, err := t.unpacker.GetRaw(request.StreamName)
	if err != nil {
		reply.Error = err.Error()
		return conn.Encode(reply)
	}
	defer reader.Close()
	reply.Size = length
	if err := conn.Encode(reply); err != nil {
		return err
	}
	_, err = io.CopyN(conn, reader, int64(length))
	return err
}
