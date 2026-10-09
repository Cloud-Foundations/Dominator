package grpc

import (
	"context"
	"crypto/tls"
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

type authInfoKeyType struct{}

var authInfoKey = authInfoKeyType{}

var publicMethods = make(map[string]struct{})
var unauthenticatedMethods = make(map[string]struct{})

// wrappedStream overrides Context to include auth info.
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}

func authInfoFromContext(ctx context.Context) *AuthInfo {
	if v := ctx.Value(authInfoKey); v != nil {
		return v.(*AuthInfo)
	}
	return nil
}

func contextWithAuthInfo(ctx context.Context,
	authInfo *AuthInfo) context.Context {
	return context.WithValue(ctx, authInfoKey, authInfo)
}

func (a *AuthInfo) getAuthInformation() *srpc.AuthInformation {
	if a == nil {
		return nil
	}
	return a.authInformation
}

func registerServiceOptions(serviceName string, options ServiceOptions) {
	allMethods := make(map[string]struct{})
	for _, method := range options.PublicMethods {
		fullMethod := "/" + serviceName + "/" + method
		publicMethods[fullMethod] = struct{}{}
		allMethods[fullMethod] = struct{}{}
	}
	for _, method := range options.UnauthenticatedMethods {
		fullMethod := "/" + serviceName + "/" + method
		unauthenticatedMethods[fullMethod] = struct{}{}
		allMethods[fullMethod] = struct{}{}
	}
	registerMethodMetrics(serviceName, allMethods)
}

func streamAuthInterceptor(srv interface{}, ss grpc.ServerStream,
	info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx, err := authoriseRequest(ss.Context(), info.FullMethod)
	if err != nil {
		return err
	}
	wrapped := &wrappedStream{ServerStream: ss, ctx: ctx}
	recordCallStart()
	startTime := time.Now()
	defer func() {
		if r := recover(); r != nil {
			recordPanic()
			panic(r)
		}
	}()
	err = handler(srv, wrapped)
	recordCallEnd(info.FullMethod, startTime, err)
	return err
}

func unaryAuthInterceptor(ctx context.Context, req interface{},
	info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	ctx, err := authoriseRequest(ctx, info.FullMethod)
	if err != nil {
		return nil, err
	}
	recordCallStart()
	startTime := time.Now()
	defer func() {
		if r := recover(); r != nil {
			recordPanic()
			panic(r)
		}
	}()
	resp, err := handler(ctx, req)
	recordCallEnd(info.FullMethod, startTime, err)
	return resp, err
}

func authoriseRequest(ctx context.Context,
	fullMethod string) (context.Context, error) {
	_, isPublic := publicMethods[fullMethod]
	_, isUnauthenticated := unauthenticatedMethods[fullMethod]
	if isUnauthenticated {
		return contextWithAuthInfo(ctx, &AuthInfo{}), nil
	}
	tlsState, err := tlsStateFromContext(ctx)
	if err != nil {
		return nil, err
	}
	authInformation, authorised, err := srpc.CheckTlsAuthorisation(
		grpcToSrpcMethod(fullMethod), tlsState,
		!doNotUseMethodPowersFromMetadata(ctx), isPublic)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	if !authorised {
		recordDeniedCall(fullMethod)
		return nil, status.Error(codes.PermissionDenied, "call on "+fullMethod)
	}
	return contextWithAuthInfo(ctx,
		&AuthInfo{authInformation: authInformation}), nil
}

func tlsStateFromContext(ctx context.Context) (tls.ConnectionState, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return tls.ConnectionState{},
			status.Error(codes.Unauthenticated, "no peer info in context")
	}
	if p.AuthInfo == nil {
		return tls.ConnectionState{},
			status.Error(codes.Unauthenticated, "no TLS auth info")
	}
	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return tls.ConnectionState{},
			status.Error(codes.Unauthenticated, "unexpected auth info type")
	}
	return tlsInfo.State, nil
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
