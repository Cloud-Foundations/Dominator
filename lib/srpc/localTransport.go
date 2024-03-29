package srpc

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"flag"
	"fmt"
	"net"
	"os"
	"runtime"
	"sync"
)

const (
	unixClientCookieLength = 8
	unixServerCookieLength = 8
	unixBufferSize         = 1 << 16
)

var (
	srpcUnixSocketPath = flag.String("srpcUnixSocketPath",
		defaultUnixSocketPath(),
		"Pathname for server Unix sockets")

	unixCookieToConnMapLock sync.Mutex
	unixCookieToConn        map[[unixServerCookieLength]byte]net.Conn
	unixListenerSetup       sync.Once
)

type localUpgradeToUnixRequestOne struct {
	ClientCookie []byte
}

type localUpgradeToUnixResponseOne struct {
	Error          string
	ServerCookie   []byte
	SocketPathname string
}

type localUpgradeToUnixRequestTwo struct {
	SentServerCookie bool
}

type localUpgradeToUnixResponseTwo struct {
	Error string
}

func acceptUnix(conn net.Conn,
	unixCookieToConn map[[unixServerCookieLength]byte]net.Conn) {
	doClose := true
	defer func() {
		if doClose {
			conn.Close()
		}
	}()
	var cookie [unixServerCookieLength]byte
	if length, err := conn.Read(cookie[:]); err != nil {
		return
	} else if length != unixServerCookieLength {
		return
	}
	unixCookieToConnMapLock.Lock()
	if _, ok := unixCookieToConn[cookie]; ok {
		unixCookieToConnMapLock.Unlock()
		return
	}
	unixCookieToConn[cookie] = conn
	unixCookieToConnMapLock.Unlock()
	var ack [1]byte
	length, err := conn.Write(ack[:])
	if err != nil || length != 1 {
		unixCookieToConnMapLock.Lock()
		delete(unixCookieToConn, cookie)
		unixCookieToConnMapLock.Unlock()
		return
	}
	doClose = false
}

func acceptUnixLoop(l net.Listener,
	unixCookieToConn map[[unixServerCookieLength]byte]net.Conn) {
	defer l.Close()
	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error accepting Unix connection: %s\n", err)
			return
		}
		go acceptUnix(conn, unixCookieToConn)
	}
}

func defaultUnixSocketPath() string {
	if runtime.GOOS != "linux" {
		return ""
	}
	return fmt.Sprintf("@SRPC.%d", os.Getpid())
}

func isLocal(client *Client) bool {
	lhost, _, err := net.SplitHostPort(client.localAddr)
	if err != nil {
		return false
	}
	rhost, _, err := net.SplitHostPort(client.remoteAddr)
	if err != nil {
		return false
	}
	return lhost == rhost
}

func setupUnixListener() {
	if *srpcUnixSocketPath == "" {
		return
	}
	if (*srpcUnixSocketPath)[0] != '@' {
		os.Remove(*srpcUnixSocketPath)
	}
	l, err := net.Listen("unix", *srpcUnixSocketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listening on Unix socket: %s\n", err)
		return
	}
	unixCookieToConn = make(map[[unixServerCookieLength]byte]net.Conn)
	go acceptUnixLoop(l, unixCookieToConn)
}

func (*builtinReceiver) LocalUpgradeToUnix(conn *Conn) error {
	unixListenerSetup.Do(setupUnixListener)
	var requestOne localUpgradeToUnixRequestOne
	if err := conn.Decode(&requestOne); err != nil {
		return err
	}
	if *srpcUnixSocketPath == "" || unixCookieToConn == nil {
		return conn.Encode(localUpgradeToUnixResponseOne{Error: "no socket"})
	}
	var cookie [unixServerCookieLength]byte
	if length, err := rand.Read(cookie[:]); err != nil {
		return conn.Encode(localUpgradeToUnixResponseOne{Error: err.Error()})
	} else if length != unixServerCookieLength {
		return conn.Encode(localUpgradeToUnixResponseOne{Error: "bad length"})
	}
	err := conn.Encode(localUpgradeToUnixResponseOne{
		ServerCookie:   cookie[:],
		SocketPathname: *srpcUnixSocketPath,
	})
	if err != nil {
		return err
	}
	if err := conn.Flush(); err != nil {
		return err
	}
	var requestTwo localUpgradeToUnixRequestTwo
	if err := conn.Decode(&requestTwo); err != nil {
		return err
	}
	if !requestTwo.SentServerCookie {
		return nil
	}
	unixCookieToConnMapLock.Lock()
	newConn, ok := unixCookieToConn[cookie]
	unixCookieToConnMapLock.Unlock()
	doClose := true
	defer func() {
		if doClose && newConn != nil {
			newConn.Close()
		}
	}()
	if !ok {
		return conn.Encode(
			localUpgradeToUnixResponseTwo{Error: "cookie not found"})
	}
	if err := conn.Encode(localUpgradeToUnixResponseTwo{}); err != nil {
		return err
	}
	if err := conn.Flush(); err != nil {
		return err
	}
	if length, err := newConn.Write(requestOne.ClientCookie); err != nil {
		return err
	} else if length != len(requestOne.ClientCookie) {
		return fmt.Errorf("could not write full client cookie")
	}
	doClose = false
	conn.conn.Close()
	conn.conn = newConn
	conn.ReadWriter = bufio.NewReadWriter(
		bufio.NewReaderSize(newConn, unixBufferSize),
		bufio.NewWriterSize(newConn, unixBufferSize))
	logger.Debugf(0, "upgraded connection from: %s to Unix\n", conn.remoteAddr)
	return nil
}

func (client *Client) localAttemptUpgradeToUnix() (bool, error) {
	if !isLocal(client) {
		return false, nil
	}
	var cookie [unixClientCookieLength]byte
	if length, err := rand.Read(cookie[:]); err != nil {
		return false, nil
	} else if length != unixClientCookieLength {
		return false, nil
	}
	conn, err := client.Call(".LocalUpgradeToUnix")
	if err != nil {
		return false, nil
	}
	defer conn.Close()
	defer conn.Flush()
	err = conn.Encode(localUpgradeToUnixRequestOne{ClientCookie: cookie[:]})
	if err != nil {
		return false, err
	}
	if err := conn.Flush(); err != nil {
		return false, err
	}
	var replyOne localUpgradeToUnixResponseOne
	if err := conn.Decode(&replyOne); err != nil {
		return false, err
	}
	if replyOne.Error != "" {
		return false, nil
	}
	newConn, err := net.Dial("unix", replyOne.SocketPathname)
	if err != nil {
		conn.Encode(localUpgradeToUnixRequestTwo{})
		logger.Println(err)
		return false, nil
	}
	doClose := true
	defer func() {
		if doClose {
			newConn.Close()
		}
	}()
	if length, err := newConn.Write(replyOne.ServerCookie); err != nil {
		conn.Encode(localUpgradeToUnixRequestTwo{})
		return false, err
	} else if length != len(replyOne.ServerCookie) {
		conn.Encode(localUpgradeToUnixRequestTwo{})
		return false, fmt.Errorf("bad cookie length: %d", length)
	}
	var ack [1]byte
	if length, err := newConn.Read(ack[:]); err != nil {
		conn.Encode(localUpgradeToUnixRequestTwo{})
		return false, err
	} else if length != 1 {
		conn.Encode(localUpgradeToUnixRequestTwo{})
		return false, fmt.Errorf("bad ack length: %d", length)
	}
	err = conn.Encode(localUpgradeToUnixRequestTwo{SentServerCookie: true})
	if err != nil {
		return false, err
	}
	if err := conn.Flush(); err != nil {
		return false, err
	}
	var replyTwo localUpgradeToUnixResponseTwo
	if err := conn.Decode(&replyTwo); err != nil {
		return false, err
	}
	if replyTwo.Error != "" {
		return false, nil
	}
	returnedClientCookie := make([]byte, len(cookie))
	if length, err := newConn.Read(returnedClientCookie); err != nil {
		return false, err
	} else if length != len(cookie) {
		return false, fmt.Errorf("bad returned cookie length: %d", length)
	}
	if !bytes.Equal(returnedClientCookie, cookie[:]) {
		return false, fmt.Errorf("returned client cookie does not match")
	}
	doClose = false
	client.conn.Close()
	client.conn = newConn
	client.connType = "Unix"
	client.tcpConn = nil
	client.bufrw = bufio.NewReadWriter(
		bufio.NewReaderSize(newConn, unixBufferSize),
		bufio.NewWriterSize(newConn, unixBufferSize))
	return true, nil
}
