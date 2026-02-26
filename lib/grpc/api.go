package grpc

import (
	"context"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

type connKeyType struct{}

var connKey = connKeyType{}

var publicMethods = make(map[string]struct{})
var unauthenticatedMethods = make(map[string]struct{})

// grpcToSrpcMethodMapping maps gRPC method names to SRPC method names per service.
var grpcToSrpcMethodMapping = make(map[string]map[string]string)

// Interface check.
var _ srpc.AuthConn = (*Conn)(nil)

// Conn holds authentication information for gRPC handlers.
type Conn struct {
	authInfo         *srpc.AuthInformation
	permittedMethods map[string]struct{}
}

// GetAuthInformation returns the authentication information or nil.
func (c *Conn) GetAuthInformation() *srpc.AuthInformation {
	if c == nil {
		return nil
	}
	return c.authInfo
}

func (c *Conn) GetPermittedMethods() map[string]struct{} {
	if c == nil {
		return nil
	}
	return c.permittedMethods
}

// AllowMethodPowers always returns true for gRPC. SRPC supports a
// "doNotUseMethodPowers" query parameter allowing clients to opt-out of
// method powers; gRPC has no equivalent mechanism.
func (c *Conn) AllowMethodPowers() bool {
	return true
}

func authorizeRequest(ctx context.Context, fullMethod string) (context.Context, error) {
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
	authorized, haveMethodAccess := srpc.CheckAuthorization(srpcMethod, conn,
		srpc.GetDefaultGrantMethod(), isPublic, false)
	if !authorized {
		recordDeniedCall(fullMethod)
		return nil, status.Error(codes.PermissionDenied, "call on "+fullMethod)
	}
	conn.GetAuthInformation().HaveMethodAccess = haveMethodAccess
	return ContextWithConn(ctx, conn), nil
}

// UnaryAuthInterceptor handles authentication and authorization for unary RPCs.
func UnaryAuthInterceptor(ctx context.Context, req interface{},
	info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {

	ctx, err := authorizeRequest(ctx, info.FullMethod)
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

// StreamAuthInterceptor handles authentication and authorization for streaming RPCs.
func StreamAuthInterceptor(srv interface{}, ss grpc.ServerStream,
	info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {

	ctx, err := authorizeRequest(ss.Context(), info.FullMethod)
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

// ConnFromContext returns the Conn from the context.
func ConnFromContext(ctx context.Context) *Conn {
	if v := ctx.Value(connKey); v != nil {
		return v.(*Conn)
	}
	return nil
}

// ContextWithConn returns a context with the Conn attached.
func ContextWithConn(ctx context.Context, conn *Conn) context.Context {
	return context.WithValue(ctx, connKey, conn)
}

// ServiceOptions configures authorization for a gRPC service.
type ServiceOptions struct {
	PublicMethods                  []string          // SRPC method names
	UnauthenticatedMethods         []string          // SRPC method names
	GrpcToSrpcMethods              map[string]string // e.g., {"ListVms": "ListVMs"}
	GrpcOnlyPublicMethods          []string          // gRPC-only public methods
	GrpcOnlyUnauthenticatedMethods []string          // gRPC-only unauthenticated methods
}

// RegisterServiceOptions registers public and unauthenticated methods for a service.
// It translates SRPC method names to gRPC equivalents and stores the mapping for RBAC.
func RegisterServiceOptions(serviceName string, options ServiceOptions) {
	srpcToGrpc := reverseMapping(options.GrpcToSrpcMethods)

	allMethods := make(map[string]struct{})
	for _, method := range translateMethods(options.PublicMethods, srpcToGrpc, options.GrpcOnlyPublicMethods) {
		fullMethod := "/" + serviceName + "/" + method
		publicMethods[fullMethod] = struct{}{}
		allMethods[fullMethod] = struct{}{}
	}
	for _, method := range translateMethods(options.UnauthenticatedMethods, srpcToGrpc, options.GrpcOnlyUnauthenticatedMethods) {
		fullMethod := "/" + serviceName + "/" + method
		unauthenticatedMethods[fullMethod] = struct{}{}
		allMethods[fullMethod] = struct{}{}
	}
	if options.GrpcToSrpcMethods != nil {
		grpcToSrpcMethodMapping[serviceName] = options.GrpcToSrpcMethods
	}
	registerMethodMetrics(serviceName, allMethods)
}

func reverseMapping(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	result := make(map[string]string, len(m))
	for k, v := range m {
		result[v] = k
	}
	return result
}

func translateMethods(srpcMethods []string, srpcToGrpc map[string]string, grpcOnly []string) []string {
	result := make([]string, 0, len(srpcMethods)+len(grpcOnly))
	for _, method := range srpcMethods {
		if mapped, ok := srpcToGrpc[method]; ok {
			result = append(result, mapped)
		} else {
			result = append(result, method)
		}
	}
	return append(result, grpcOnly...)
}

// grpcToSrpcMethod converts "/hypervisor.Hypervisor/ListVms" to "Hypervisor.ListVMs".
func grpcToSrpcMethod(fullMethod string) string {
	parts := strings.Split(strings.TrimPrefix(fullMethod, "/"), "/")
	if len(parts) != 2 {
		return fullMethod
	}
	servicePart := parts[0]
	methodName := parts[1]

	serviceParts := strings.Split(servicePart, ".")
	serviceName := serviceParts[len(serviceParts)-1]

	if methodMap, ok := grpcToSrpcMethodMapping[servicePart]; ok {
		if srpcMethod, ok := methodMap[methodName]; ok {
			methodName = srpcMethod
		}
	}

	return serviceName + "." + methodName
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
		permittedMethods: permittedMethods,
	}, nil
}

// wrappedStream overrides Context to include auth info.
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}

// Metrics stubs - replaced by metrics.go in a later PR.
func recordDeniedCall(fullMethod string)                                    {}
func recordCallStart()                                                      {}
func recordCallEnd(fullMethod string, startTime time.Time, err error)       {}
func recordPanic()                                                          {}
func registerMethodMetrics(serviceName string, methods map[string]struct{}) {}
