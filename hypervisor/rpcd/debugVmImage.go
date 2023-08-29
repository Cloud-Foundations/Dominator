package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

func (t *srpcType) DebugVmImage(conn *srpc.Conn) error {
	return t.manager.DebugVmImage(conn, conn.GetAuthInformation())
}
