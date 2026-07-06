package grpc

import (
	"context"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

type connKeyType struct{}

var connKey = connKeyType{}

var publicMethods = make(map[string]struct{})
var unauthenticatedMethods = make(map[string]struct{})

// Interface check.
var _ srpc.AuthConn = (*Conn)(nil)

// wrappedStream overrides Context to include auth info.
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}

func authoriseRequest(ctx context.Context, fullMethod string) (context.Context, error) {
	_, isPublic := publicMethods[fullMethod]
	_, isUnauthenticated := unauthenticatedMethods[fullMethod]
	if isUnauthenticated {
		return ContextWithConn(ctx, &Conn{}), nil
	}
	conn, err := buildAuthConn(ctx)
	if err != nil {
		return nil, err
	}
	if conn.GetAuthInformation() == nil {
		return nil, status.Error(codes.Unauthenticated, "no auth information")
	}
	srpcMethod := grpcToSrpcMethod(fullMethod)
	authorised, haveMethodAccess := srpc.CheckAuthorisation(srpcMethod, conn,
		srpc.GetDefaultGrantMethod(), isPublic, false)
	if !authorised {
		recordDeniedCall(fullMethod)
		return nil, status.Error(codes.PermissionDenied, "call on "+fullMethod)
	}
	conn.GetAuthInformation().HaveMethodAccess = haveMethodAccess
	return ContextWithConn(ctx, conn), nil
}

func buildAuthConn(ctx context.Context) (*Conn, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "no peer info in context")
	}
	if p.AuthInfo == nil {
		return nil, status.Error(codes.Unauthenticated, "no TLS auth info")
	}
	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unexpected auth info type")
	}
	username, permittedMethods, groupList, err := srpc.GetAuth(tlsInfo.State)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return &Conn{
		authInfo: &srpc.AuthInformation{
			Username:  username,
			GroupList: groupList,
		},
		permittedMethods:  permittedMethods,
		allowMethodPowers: !doNotUseMethodPowersFromMetadata(ctx),
	}, nil
}

// doNotUseMethodPowersFromMetadata returns true if the incoming request
// carries metadata opting out of method powers.
func doNotUseMethodPowersFromMetadata(ctx context.Context) bool {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}
	for _, v := range md.Get(DoNotUseMethodPowersMetadataKey) {
		if v == "true" {
			return true
		}
	}
	return false
}

// grpcToSrpcMethod converts "/package.Service/Method" to "Service.Method".
func grpcToSrpcMethod(fullMethod string) string {
	parts := strings.Split(strings.TrimPrefix(fullMethod, "/"), "/")
	if len(parts) != 2 {
		return fullMethod
	}
	serviceParts := strings.Split(parts[0], ".")
	serviceName := serviceParts[len(serviceParts)-1]
	return serviceName + "." + parts[1]
}

// Metrics stubs - replaced by metrics.go in a later PR.
func recordDeniedCall(fullMethod string)                                    {}
func recordCallStart()                                                      {}
func recordCallEnd(fullMethod string, startTime time.Time, err error)       {}
func recordPanic()                                                          {}
func registerMethodMetrics(serviceName string, methods map[string]struct{}) {}
