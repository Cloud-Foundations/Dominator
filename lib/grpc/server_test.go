package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// TestServerRefusesUnauthenticatedCalls checks that unary and streaming
// calls without a TLS client identity are refused, which shows that both
// interceptors are installed.
func TestServerRefusesUnauthenticatedCalls(t *testing.T) {
	listener := bufconn.Listen(1 << 16)
	server := newServer()
	healthpb.RegisterHealthServer(server, health.NewServer())
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context,
			string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := healthpb.NewHealthClient(conn)
	_, err = client.Check(ctx, &healthpb.HealthCheckRequest{})
	if code := status.Code(err); code != codes.Unauthenticated {
		t.Errorf("unary call: got code %v, want Unauthenticated", code)
	}
	stream, err := client.Watch(ctx, &healthpb.HealthCheckRequest{})
	if err == nil {
		_, err = stream.Recv()
	}
	if code := status.Code(err); code != codes.Unauthenticated {
		t.Errorf("streaming call: got code %v, want Unauthenticated", code)
	}
}

func TestDefaultServerIsShared(t *testing.T) {
	first := DefaultServer()
	if DefaultServer() != first {
		t.Error("DefaultServer returned different servers")
	}
}
