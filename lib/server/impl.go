package server

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/http2"
	"google.golang.org/grpc"

	lib_grpc "github.com/Cloud-Foundations/Dominator/lib/grpc"
	libnet "github.com/Cloud-Foundations/Dominator/lib/net"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

const (
	grpcContentType        = "application/grpc"
	tlsHandshakeRecordType = 0x16
)

// detectTimeout is how long a new connection may take to send its first byte.
// Tests shorten it.
var detectTimeout = 10 * time.Second

// bufferedConn is a net.Conn which reads through a buffer that has already
// been peeked into. It implements libnet.TCPConn so that SRPC still enables
// keepalive on the connections it takes over.
type bufferedConn struct {
	net.Conn
	reader  *bufio.Reader
	tcpConn libnet.TCPConn // If nil: the keepalive methods do nothing.
}

// detectingListener is a net.Listener which returns TLS connections that have
// been wrapped with tlsConfig and other connections unchanged.
type detectingListener struct {
	net.Listener
	tlsConfig *tls.Config
	ready     chan net.Conn
	done      chan struct{} // Closed after err is set.
	err       error
}

// start returns an error if the port cannot be listened on, otherwise it
// returns the error which stopped serving.
func start(config Config, params Params) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", config.Port))
	if err != nil {
		return fmt.Errorf("cannot create listener: %w", err)
	}
	if params.Logger != nil {
		params.Logger.Printf("Serving SRPC, HTML and gRPC on port %d\n",
			config.Port)
	}
	return serveListener(listener, params)
}

// serveListener returns the error which stopped serving on listener, which it
// closes. It serves gRPC with the default gRPC server if params has no gRPC
// handler and services are registered on it.
func serveListener(listener net.Listener, params Params) error {
	params.GrpcHandler = grpcHandler(params.GrpcHandler,
		lib_grpc.DefaultServer())
	return serve(listener, srpc.GetServerTlsConfig(), params)
}

// grpcHandler returns handler if it is not nil, otherwise defaultServer if any
// services are registered on it, otherwise nil.
func grpcHandler(handler http.Handler,
	defaultServer *grpc.Server) http.Handler {
	if handler != nil {
		return handler
	}
	if len(defaultServer.GetServiceInfo()) > 0 {
		return defaultServer
	}
	return nil
}

// serve returns the error which stopped serving on listener, which it closes.
// It modifies tlsConfig, which the caller must not use afterwards. If there is
// no gRPC handler it serves as an SRPC server does without this package. If
// tlsConfig is nil gRPC is served over cleartext HTTP/2 (h2c).
func serve(listener net.Listener, tlsConfig *tls.Config, params Params) error {
	if params.GrpcHandler == nil {
		return http.Serve(listener, params.HttpHandler)
	}
	httpHandler := params.HttpHandler
	if httpHandler == nil {
		httpHandler = http.DefaultServeMux
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter,
		req *http.Request) {
		if strings.HasPrefix(req.Header.Get("Content-Type"),
			grpcContentType) {
			params.GrpcHandler.ServeHTTP(w, req)
		} else {
			httpHandler.ServeHTTP(w, req)
		}
	})
	if tlsConfig == nil {
		server := &http.Server{Handler: handler, Protocols: new(http.Protocols)}
		server.Protocols.SetHTTP1(true)
		server.Protocols.SetUnencryptedHTTP2(true)
		return server.Serve(listener)
	}
	tlsConfig.ClientAuth = tls.VerifyClientCertIfGiven
	tlsConfig.NextProtos = append(tlsConfig.NextProtos, "h2", "http/1.1")
	server := &http.Server{Handler: handler}
	if err := http2.ConfigureServer(server, &http2.Server{}); err != nil {
		_ = listener.Close()
		return fmt.Errorf("cannot configure HTTP/2: %w", err)
	}
	return server.Serve(newDetectingListener(listener, tlsConfig))
}

// newDetectingListener returns a listener which accepts from listener. It
// starts a goroutine which runs until listener fails.
func newDetectingListener(listener net.Listener,
	tlsConfig *tls.Config) *detectingListener {
	l := &detectingListener{
		Listener:  listener,
		tlsConfig: tlsConfig,
		ready:     make(chan net.Conn),
		done:      make(chan struct{}),
	}
	go l.acceptLoop()
	return l
}

// Accept returns the next classified connection, or the error which stopped
// the underlying listener.
func (l *detectingListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.ready:
		return conn, nil
	case <-l.done:
		return nil, l.err
	}
}

// acceptLoop accepts connections until the underlying listener fails. Each
// connection is classified in its own goroutine so that a client which sends
// nothing cannot stall the others.
func (l *detectingListener) acceptLoop() {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			l.err = err
			close(l.done)
			return
		}
		go l.classify(conn)
	}
}

// classify peeks at the first byte of conn and hands it to Accept, wrapped
// with TLS if it starts a handshake. It closes conn if nothing arrives within
// detectTimeout.
func (l *detectingListener) classify(conn net.Conn) {
	bufConn := &bufferedConn{Conn: conn, reader: bufio.NewReader(conn)}
	bufConn.tcpConn, _ = conn.(libnet.TCPConn)
	err := conn.SetReadDeadline(time.Now().Add(detectTimeout))
	if err != nil {
		_ = conn.Close()
		return
	}
	firstByte, err := bufConn.reader.Peek(1)
	if err == nil {
		err = conn.SetReadDeadline(time.Time{})
	}
	if err != nil {
		_ = conn.Close()
		return
	}
	var result net.Conn = bufConn
	if firstByte[0] == tlsHandshakeRecordType {
		result = tls.Server(bufConn, l.tlsConfig)
	}
	select {
	case l.ready <- result:
	case <-l.done:
		_ = result.Close()
	}
}

// Read reads from the buffer, which holds any bytes already peeked. It
// returns the number of bytes read and any error, as io.Reader does.
func (c *bufferedConn) Read(b []byte) (int, error) {
	return c.reader.Read(b)
}

// SetKeepAlive sets keepalive on the underlying connection if it is TCP. It
// returns the error from the underlying connection, or nil if it is not TCP.
func (c *bufferedConn) SetKeepAlive(keepalive bool) error {
	if c.tcpConn == nil {
		return nil
	}
	return c.tcpConn.SetKeepAlive(keepalive)
}

// SetKeepAlivePeriod sets the keepalive period on the underlying connection if
// it is TCP. It returns the error from the underlying connection, or nil if it
// is not TCP.
func (c *bufferedConn) SetKeepAlivePeriod(d time.Duration) error {
	if c.tcpConn == nil {
		return nil
	}
	return c.tcpConn.SetKeepAlivePeriod(d)
}
