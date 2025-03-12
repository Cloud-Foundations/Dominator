package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/sub"
)

func (t *rpcType) BoostScanLimit(conn *srpc.Conn,
	request sub.BoostScanLimitRequest,
	reply *sub.BoostScanLimitResponse) error {
	t.params.ScannerConfiguration.BoostScanLimit(t.params.Logger)
	return nil
}
