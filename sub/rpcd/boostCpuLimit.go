package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/sub"
)

func (t *rpcType) BoostCpuLimit(conn *srpc.Conn,
	request sub.BoostCpuLimitRequest, reply *sub.BoostCpuLimitResponse) error {
	t.params.ScannerConfiguration.BoostCpuLimit(t.params.Logger)
	return nil
}
