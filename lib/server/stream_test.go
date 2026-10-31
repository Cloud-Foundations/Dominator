package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/mem"
)

const (
	downloadMessages = 200
	downloadSize     = 64 << 10
)

// rawCodec sends []byte messages unchanged, so the tests need no protobuf
// message types. Messages are passed to RecvMsg and SendMsg as *[]byte.
type rawCodec struct{}

// Marshal returns the bytes that v, a *[]byte, points to.
func (rawCodec) Marshal(v any) (mem.BufferSlice, error) {
	return mem.BufferSlice{mem.SliceBuffer(*v.(*[]byte))}, nil
}

// Unmarshal stores a copy of data in v, a *[]byte.
func (rawCodec) Unmarshal(data mem.BufferSlice, v any) error {
	*v.(*[]byte) = data.Materialize()
	return nil
}

// Name returns the content subtype, which makes requests "application/grpc+raw".
func (rawCodec) Name() string {
	return "raw"
}

// echoStream echoes every message back until the client closes its side.
func echoStream(_ any, stream grpc.ServerStream) error {
	for {
		var message []byte
		if err := stream.RecvMsg(&message); err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		if err := stream.SendMsg(&message); err != nil {
			return err
		}
	}
}

// downloadStream reads a count and then sends that many large messages.
func downloadStream(_ any, stream grpc.ServerStream) error {
	var request []byte
	if err := stream.RecvMsg(&request); err != nil {
		return err
	}
	count, err := strconv.Atoi(string(request))
	if err != nil {
		return err
	}
	chunk := make([]byte, downloadSize)
	for i := 0; i < count; i++ {
		if err := stream.SendMsg(&chunk); err != nil {
			return err
		}
	}
	return nil
}

// startGrpcStreamServer serves a real grpc.Server with the two streaming
// methods above and returns a client connection to it over TLS.
func startGrpcStreamServer(t *testing.T) *grpc.ClientConn {
	grpcServer := grpc.NewServer(grpc.ForceServerCodecV2(rawCodec{}))
	grpcServer.RegisterService(&grpc.ServiceDesc{
		ServiceName: "test.Stream",
		HandlerType: (*any)(nil),
		Streams: []grpc.StreamDesc{
			{StreamName: "Echo", Handler: echoStream, ServerStreams: true,
				ClientStreams: true},
			{StreamName: "Download", Handler: downloadStream,
				ServerStreams: true},
		},
	}, struct{}{})
	addr := startServer(t, Params{GrpcHandler: grpcServer})
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(
			credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})),
		grpc.WithDefaultCallOptions(grpc.ForceCodecV2(rawCodec{})))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// echoRounds sends rounds messages on a new Echo stream, waiting for each
// echo, and pausing for idle after the first round. It returns the first
// error.
func echoRounds(ctx context.Context, conn *grpc.ClientConn, name string,
	rounds int, idle time.Duration) error {
	stream, err := conn.NewStream(ctx, &grpc.StreamDesc{
		ServerStreams: true, ClientStreams: true}, "/test.Stream/Echo")
	if err != nil {
		return err
	}
	for i := 0; i < rounds; i++ {
		sent := []byte(fmt.Sprintf("%s-%d", name, i))
		if err := stream.SendMsg(&sent); err != nil {
			return err
		}
		var got []byte
		if err := stream.RecvMsg(&got); err != nil {
			return fmt.Errorf("%s round %d: %w", name, i, err)
		}
		if string(got) != string(sent) {
			return fmt.Errorf("%s: got %q, want %q", name, got, sent)
		}
		if i == 0 {
			time.Sleep(idle)
		}
	}
	if err := stream.CloseSend(); err != nil {
		return err
	}
	var extra []byte
	if err := stream.RecvMsg(&extra); err != io.EOF {
		return fmt.Errorf("%s: want EOF after CloseSend, got %v", name, err)
	}
	return nil
}

// Many bidirectional streams share one connection, and each idles for longer
// than the first-byte timeout, which must not close the connection.
func TestGrpcBidirectionalStreamsMultiplexAndIdle(t *testing.T) {
	conn := startGrpcStreamServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	const streams = 10
	errs := make(chan error, streams)
	var wg sync.WaitGroup
	for i := 0; i < streams; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- echoRounds(ctx, conn, fmt.Sprintf("stream%d", i), 20,
				2*detectTimeout)
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
}

// A large server-to-client stream exercises HTTP/2 flow control through the
// wrapped listener.
func TestGrpcLargeServerStream(t *testing.T) {
	conn := startGrpcStreamServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	stream, err := conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true},
		"/test.Stream/Download")
	if err != nil {
		t.Fatal(err)
	}
	request := []byte(strconv.Itoa(downloadMessages))
	if err := stream.SendMsg(&request); err != nil {
		t.Fatal(err)
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatal(err)
	}
	var total int
	for {
		var chunk []byte
		if err := stream.RecvMsg(&chunk); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("after %d bytes: %s", total, err)
		}
		total += len(chunk)
	}
	if want := downloadMessages * downloadSize; total != want {
		t.Fatalf("received %d bytes, want %d", total, want)
	}
}

// A connection which a handler has taken over, which is how SRPC carries a
// long call, keeps working after the first-byte timeout has passed.
func TestTakenOverStreamSurvivesIdle(t *testing.T) {
	addr := startServer(t, Params{
		GrpcHandler: http.NotFoundHandler(),
		HttpHandler: http.HandlerFunc(func(w http.ResponseWriter,
			req *http.Request) {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				return
			}
			defer func() { _ = conn.Close() }()
			_, _ = io.WriteString(conn, "HTTP/1.0 200 Connected\r\n\r\n")
			_, _ = io.Copy(conn, conn)
		}),
	})
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	reader := bufio.NewReader(conn)
	_, err = io.WriteString(conn, "CONNECT /x HTTP/1.1\r\nHost: x\r\n\r\n")
	if err != nil {
		t.Fatal(err)
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	for i := 0; i < 3; i++ {
		time.Sleep(2 * detectTimeout)
		want := fmt.Sprintf("line %d\n", i)
		if _, err := io.WriteString(conn, want); err != nil {
			t.Fatal(err)
		}
		if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatal(err)
		}
		got, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("round %d: %s", i, err)
		}
		if got != want {
			t.Fatalf("round %d: got %q, want %q", i, got, want)
		}
	}
}
