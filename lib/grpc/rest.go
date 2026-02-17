package grpc

import (
	"net/http"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

type RestAuthConfig struct {
	ServiceName  string
	PathToMethod func(path, httpMethod string) string
}

// NewRestAuthMiddleware creates HTTP middleware for REST API authentication.
func NewRestAuthMiddleware(config RestAuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			method := config.PathToMethod(r.URL.Path, r.Method)
			fullMethod := "/" + config.ServiceName + "/" + method
			startTime := time.Now()

			if method == "Unknown" {
				http.Error(w, "endpoint not found: "+r.URL.Path, http.StatusNotFound)
				return
			}

			if _, ok := unauthenticatedMethods[fullMethod]; ok {
				ctx := ContextWithConn(r.Context(), &Conn{})
				recordRestCallStart()
				next.ServeHTTP(w, r.WithContext(ctx))
				recordRestCallEnd(fullMethod, startTime, nil)
				return
			}

			if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
				recordRestDeniedCall(fullMethod)
				http.Error(w, "client certificate required", http.StatusUnauthorized)
				return
			}
			username, permittedMethods, groupList, err := srpc.GetAuth(*r.TLS)
			if err != nil {
				recordRestDeniedCall(fullMethod)
				http.Error(w, "authentication failed: "+err.Error(), http.StatusUnauthorized)
				return
			}

			authInfo := &srpc.AuthInformation{
				Username:  username,
				GroupList: groupList,
			}
			conn := &Conn{
				authInfo:         authInfo,
				permittedMethods: permittedMethods,
			}
			if !checkAuthorizationWithConn(fullMethod, conn) {
				http.Error(w, "permission denied: "+fullMethod, http.StatusForbidden)
				return
			}

			ctx := ContextWithConn(r.Context(), conn)
			recordRestCallStart()
			next.ServeHTTP(w, r.WithContext(ctx))
			recordRestCallEnd(fullMethod, startTime, nil)
		})
	}
}

func checkAuthorizationWithConn(fullMethod string, conn *Conn) bool {
	_, isPublic := publicMethods[fullMethod]
	srpcMethod := grpcToSrpcMethod(fullMethod)
	authorized, haveMethodAccess := srpc.CheckAuthorization(srpcMethod, conn,
		srpc.GetDefaultGrantMethod(), isPublic, false)
	if !authorized {
		recordRestDeniedCall(fullMethod)
		return false
	}
	conn.authInfo.HaveMethodAccess = haveMethodAccess
	return true
}
