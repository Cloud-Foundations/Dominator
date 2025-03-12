package srpc

import (
	"io"
	"time"
)

func (conn *Conn) close() error {
	err := conn.Flush()
	if client := conn.parent; client != nil {
		if client.timeout > 0 {
			client.conn.SetDeadline(time.Time{})
		}
		client.callLock.Unlock()
	}
	return err
}

func (conn *Conn) getAuthInformation() *AuthInformation {
	if conn.parent != nil {
		panic("cannot call GetAuthInformation() for client connection")
	}
	return &AuthInformation{
		GroupList:        conn.groupList,
		HaveMethodAccess: conn.haveMethodAccess,
		Username:         conn.username,
	}
}

func (conn *Conn) getCloseNotifier() <-chan error {
	closeChannel := make(chan error, 1)
	go func() {
		for {
			buf := make([]byte, 1)
			if _, err := conn.Read(buf); err != nil {
				if err == io.EOF {
					err = nil
				}
				closeChannel <- err
				return
			}
		}
	}()
	return closeChannel
}

func (conn *Conn) getUsername() string {
	if conn.parent != nil {
		panic("cannot call GetUsername() for client connection")
	}
	return conn.username
}
