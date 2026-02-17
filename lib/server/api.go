package server

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"

	lib_grpc "github.com/Cloud-Foundations/Dominator/lib/grpc"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

type Config struct {
	Port        uint
	Logger      log.DebugLogger
	HttpHandler http.Handler // Required. Serves SRPC and HTML.
	GrpcHandler func(grpcServer *grpc.Server, gatewayMux *runtime.ServeMux) (http.Handler, error)
}

// Start starts a server. Routes by content-type and path. Blocks.
func Start(config Config) error {
	if config.HttpHandler == nil {
		return fmt.Errorf("HttpHandler is required")
	}
	tlsConfig := srpc.GetServerTlsConfig()
	if tlsConfig == nil {
		return fmt.Errorf("TLS config not available")
	}
	if config.GrpcHandler != nil {
		tlsConfig.ClientAuth = tls.VerifyClientCertIfGiven
		tlsConfig.NextProtos = append(tlsConfig.NextProtos, "h2", "http/1.1")
	}
	tcpListener, err := net.Listen("tcp", fmt.Sprintf(":%d", config.Port))
	if err != nil {
		return fmt.Errorf("cannot create listener: %w", err)
	}
	tlsListener := tls.NewListener(tcpListener, tlsConfig)

	handler, err := buildHandler(config)
	if err != nil {
		return err
	}

	server := &http.Server{Handler: handler}
	if err := http2.ConfigureServer(server, &http2.Server{}); err != nil {
		return fmt.Errorf("failed to configure HTTP/2: %w", err)
	}

	if config.Logger != nil {
		if config.GrpcHandler != nil {
			config.Logger.Printf("Started server on port %d (gRPC+REST+HTTP)\n", config.Port)
		} else {
			config.Logger.Printf("Started HTTP server on port %d\n", config.Port)
		}
	}

	return server.Serve(tlsListener)
}

func buildHandler(config Config) (http.Handler, error) {
	if config.GrpcHandler == nil {
		return config.HttpHandler, nil
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(lib_grpc.UnaryAuthInterceptor),
		grpc.StreamInterceptor(lib_grpc.StreamAuthInterceptor),
	)
	reflection.Register(grpcServer)

	gatewayMux := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
			MarshalOptions: protojson.MarshalOptions{
				EmitUnpopulated: false,
			},
			UnmarshalOptions: protojson.UnmarshalOptions{
				DiscardUnknown: true,
			},
		}),
	)
	restHandler, err := config.GrpcHandler(grpcServer, gatewayMux)
	if err != nil {
		return nil, fmt.Errorf("failed to register services: %w", err)
	}
	if restHandler == nil {
		restHandler = gatewayMux
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc"):
			grpcServer.ServeHTTP(w, r)
		case isRestApiPath(r.URL.Path):
			restHandler.ServeHTTP(w, r)
		default:
			config.HttpHandler.ServeHTTP(w, r)
		}
	}), nil
}

func isRestApiPath(path string) bool {
	return strings.HasPrefix(path, "/v1/")
}
