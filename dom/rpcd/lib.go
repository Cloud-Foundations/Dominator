package rpcd

import (
	"fmt"
	"net"
	"strconv"

	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

func updateAuth(conn *srpc.Conn, targetHostname string,
	authInfo *srpc.AuthInformation) error {
	if authInfo.HaveMethodAccess {
		return nil
	}
	if authInfo.Username != "" { // If authenticated, rely on identity.
		return nil
	}
	remoteHost, remotePort, err := net.SplitHostPort(conn.RemoteAddr())
	if err != nil {
		return err
	}
	if portNumber, err := strconv.Atoi(remotePort); err != nil {
		return err
	} else if portNumber > 1023 {
		return nil
	}
	if remoteHost == targetHostname {
		authInfo.HaveMethodAccess = true
		authInfo.Username = fmt.Sprintf("root@%s", targetHostname)
	}
	addrs, err := net.LookupHost(targetHostname)
	if err != nil {
		return err
	}
	for _, addr := range addrs {
		if remoteHost == addr {
			authInfo.HaveMethodAccess = true
			authInfo.Username = fmt.Sprintf("root@%s", targetHostname)
			return nil
		}
	}
	return nil
}
