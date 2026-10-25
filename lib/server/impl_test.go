package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"math/big"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/http2"

	libnet "github.com/Cloud-Foundations/Dominator/lib/net"
)

// clientTimeout bounds every test request, so a broken listener fails a test
// instead of hanging it.
const clientTimeout = 10 * time.Second

// plainClient is an HTTP client for the plain-text port.
var plainClient = &http.Client{Timeout: clientTimeout}

// A short first-byte timeout makes every test exercise the deadline reset: a
// connection that outlives it must keep working. The default mux handler is
// for the nil-HttpHandler test.
func init() {
	detectTimeout = 300 * time.Millisecond
	// Registered once: a repeated test run must not register it twice.
	http.HandleFunc("/nilHttpHandlerTest", func(w http.ResponseWriter,
		req *http.Request) {
		_, _ = io.WriteString(w, "default mux")
	})
}

// makeTlsConfig returns a server TLS config with a throwaway self-signed
// certificate for localhost.
func makeTlsConfig(t *testing.T) *tls.Config {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template,
		&key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{der},
			PrivateKey: key}},
	}
}

// startServer serves params on a new localhost listener until the test ends.
// It returns the listener's address.
func startServer(t *testing.T, params Params) string {
	return startServerWithTls(t, makeTlsConfig(t), params)
}

// startServerWithTls is like startServer, with the given TLS config, which may
// be nil.
func startServerWithTls(t *testing.T, tlsConfig *tls.Config,
	params Params) string {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() { _ = serve(listener, tlsConfig, params) }()
	return listener.Addr().String()
}

// startTestServer serves handlers which reply "grpc" or "http", so a test can
// tell which one a request reached. It returns the server's address.
func startTestServer(t *testing.T) string {
	return startServer(t, Params{
		GrpcHandler: http.HandlerFunc(func(w http.ResponseWriter,
			req *http.Request) {
			_, _ = io.WriteString(w, "grpc")
		}),
		HttpHandler: http.HandlerFunc(func(w http.ResponseWriter,
			req *http.Request) {
			_, _ = io.WriteString(w, "http")
		}),
	})
}

// get sends a GET with the given content type (if any) and returns the reply
// body and the HTTP major version.
func get(t *testing.T, client *http.Client, url,
	contentType string) (string, int) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body), resp.ProtoMajor
}

// newTlsClient returns an HTTP client which accepts any server certificate
// and may use HTTP/2.
func newTlsClient() *http.Client {
	return &http.Client{
		Timeout: clientTimeout,
		Transport: &http.Transport{
			ForceAttemptHTTP2: true,
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		},
	}
}

func TestPlainHttpPassesThrough(t *testing.T) {
	addr := startTestServer(t)
	body, _ := get(t, plainClient, "http://"+addr+"/", "")
	if body != "http" {
		t.Fatalf("plain GET: got %q, want http", body)
	}
	body, _ = get(t, plainClient, "http://"+addr+"/",
		"application/grpc")
	if body != "grpc" {
		t.Fatalf("plain gRPC type: got %q, want grpc", body)
	}
}

func TestTakenOverConnSupportsKeepAlive(t *testing.T) {
	result := make(chan bool, 1)
	addr := startServer(t, Params{
		GrpcHandler: http.NotFoundHandler(),
		HttpHandler: http.HandlerFunc(func(w http.ResponseWriter,
			req *http.Request) {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				result <- false
				return
			}
			defer func() { _ = conn.Close() }()
			tcpConn, ok := conn.(libnet.TCPConn)
			result <- ok && tcpConn.SetKeepAlive(true) == nil
		}),
	})
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	_, err = io.WriteString(conn, "CONNECT /x HTTP/1.1\r\nHost: x\r\n\r\n")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case ok := <-result:
		if !ok {
			t.Fatal("taken-over conn does not support keepalive")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handler not reached")
	}
}

func TestTlsHttp2RoutedByContentType(t *testing.T) {
	addr := startTestServer(t)
	client := newTlsClient()
	body, proto := get(t, client, "https://"+addr+"/",
		"application/grpc+proto")
	if body != "grpc" || proto != 2 {
		t.Fatalf("TLS gRPC: got %q over HTTP/%d, want grpc over HTTP/2",
			body, proto)
	}
	body, _ = get(t, client, "https://"+addr+"/", "text/html")
	if body != "http" {
		t.Fatalf("TLS HTML: got %q, want http", body)
	}
}

func TestSilentConnectionDoesNotStallOthers(t *testing.T) {
	addr := startTestServer(t)
	silent, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = silent.Close() }()
	done := make(chan string, 1)
	go func() {
		resp, err := plainClient.Get("http://" + addr + "/")
		if err != nil {
			done <- err.Error()
			return
		}
		defer func() { _ = resp.Body.Close() }()
		body, _ := io.ReadAll(resp.Body)
		done <- string(body)
	}()
	select {
	case body := <-done:
		if !strings.HasPrefix(body, "http") {
			t.Fatalf("got %q, want http", body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("silent connection stalled accept")
	}
}

func TestNilHttpHandlerServesDefaultServeMux(t *testing.T) {
	addr := startServer(t, Params{GrpcHandler: http.NotFoundHandler()})
	body, _ := get(t, plainClient,
		"http://"+addr+"/nilHttpHandlerTest", "")
	if body != "default mux" {
		t.Fatalf("got %q, want default mux", body)
	}
}

func TestNilGrpcHandlerServesPlainHttpOnly(t *testing.T) {
	addr := startServer(t, Params{
		HttpHandler: http.HandlerFunc(func(w http.ResponseWriter,
			req *http.Request) {
			_, _ = io.WriteString(w, "http")
		}),
	})
	body, _ := get(t, plainClient, "http://"+addr+"/", "application/grpc")
	if body != "http" {
		t.Fatalf("plain GET: got %q, want http", body)
	}
	resp, err := newTlsClient().Get("https://" + addr + "/")
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("TLS connection accepted without a gRPC handler")
	}
}

func TestNilTlsConfigServesGrpcOverH2c(t *testing.T) {
	addr := startServerWithTls(t, nil, Params{
		GrpcHandler: http.HandlerFunc(func(w http.ResponseWriter,
			req *http.Request) {
			_, _ = io.WriteString(w, "grpc")
		}),
		HttpHandler: http.HandlerFunc(func(w http.ResponseWriter,
			req *http.Request) {
			_, _ = io.WriteString(w, "http")
		}),
	})
	h2cClient := &http.Client{
		Timeout: clientTimeout,
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network,
				addr string, _ *tls.Config) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, addr)
			},
		},
	}
	body, proto := get(t, h2cClient, "http://"+addr+"/", "application/grpc")
	if body != "grpc" || proto != 2 {
		t.Fatalf("h2c gRPC: got %q over HTTP/%d, want grpc over HTTP/2",
			body, proto)
	}
	body, _ = get(t, plainClient, "http://"+addr+"/", "")
	if body != "http" {
		t.Fatalf("plain GET: got %q, want http", body)
	}
}

func TestNilTlsConfigSupportsTakeOver(t *testing.T) {
	result := make(chan bool, 1)
	addr := startServerWithTls(t, nil, Params{
		GrpcHandler: http.NotFoundHandler(),
		HttpHandler: http.HandlerFunc(func(w http.ResponseWriter,
			req *http.Request) {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				_ = conn.Close()
			}
			result <- err == nil
		}),
	})
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	_, err = io.WriteString(conn, "CONNECT /x HTTP/1.1\r\nHost: x\r\n\r\n")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case ok := <-result:
		if !ok {
			t.Fatal("cannot take over the connection")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handler not reached")
	}
}
