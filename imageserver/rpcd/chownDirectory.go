package rpcd

import (
	"errors"
	"os/user"

	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func (t *srpcType) ChownDirectory(conn *srpc.Conn,
	request imageserver.ChangeOwnerRequest,
	reply *imageserver.ChangeOwnerResponse) error {
	if err := t.checkMutability(); err != nil {
		return err
	}
	username := conn.Username()
	if username == "" {
		return errors.New("no username: unauthenticated connection")
	}
	if request.OwnerGroup != "" {
		if _, err := user.LookupGroup(request.OwnerGroup); err != nil {
			t.logger.Printf("Error looking up group: \"%s\": %s, ignoring\n",
				request.OwnerGroup, err)
		}
	}
	t.logger.Printf("ChownDirectory(%s) to: \"%s\" by %s\n",
		request.DirectoryName, request.OwnerGroup, username)
	return t.imageDataBase.ChownDirectory(request.DirectoryName,
		request.OwnerGroup, conn.GetAuthInformation())
}
