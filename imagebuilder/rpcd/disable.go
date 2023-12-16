package rpcd

import (
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/imaginator"
)

func (t *srpcType) DisableAutoBuilds(conn *srpc.Conn,
	request proto.DisableAutoBuildsRequest,
	reply *proto.DisableAutoBuildsResponse) error {
	disabledUntil, err := t.builder.DisableAutoBuilds(request.DisableFor)
	if err != nil {
		reply.Error = err.Error()
	} else if authInfo := conn.GetAuthInformation(); authInfo != nil {
		reply.DisabledUntil = disabledUntil
		t.logger.Printf(
			"Disable(%s): auto builds until %s (%s)\n",
			authInfo.Username,
			disabledUntil.Format(format.TimeFormatSeconds),
			format.Duration(time.Until(disabledUntil)))
	}
	return nil
}

func (t *srpcType) DisableBuildRequests(conn *srpc.Conn,
	request proto.DisableBuildRequestsRequest,
	reply *proto.DisableBuildRequestsResponse) error {
	disabledUntil, err := t.builder.DisableBuildRequests(request.DisableFor)
	if err != nil {
		reply.Error = err.Error()
	} else if authInfo := conn.GetAuthInformation(); authInfo != nil {
		reply.DisabledUntil = disabledUntil
		t.logger.Printf(
			"Disable(%s): build requests until %s (%s)\n",
			authInfo.Username,
			disabledUntil.Format(format.TimeFormatSeconds),
			format.Duration(time.Until(disabledUntil)))
	}
	return nil
}
