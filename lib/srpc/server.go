package srpc

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/net"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
	"github.com/Cloud-Foundations/Dominator/lib/x509util"
	"github.com/Cloud-Foundations/tricorder/go/tricorder"
	"github.com/Cloud-Foundations/tricorder/go/tricorder/units"
)

const (
	connectString  = "200 Connected to Go SRPC"
	rpcPath        = "/_goSRPC_/"      // Unsecured endpoint. GOB coder.
	tlsRpcPath     = "/_go_TLS_SRPC_/" // Secured endpoint. GOB coder.
	jsonRpcPath    = "/_SRPC_/unsecured/JSON"
	jsonTlsRpcPath = "/_SRPC_/TLS/JSON"

	getHostnamePath       = rpcPath + "getHostname"
	listMethodsPath       = rpcPath + "listMethods"
	listPublicMethodsPath = rpcPath + "listPublicMethods"

	doNotUseMethodPowers = "doNotUseMethodPowers"

	methodTypeRaw = iota
	methodTypeCoder
	methodTypeRequestReply
)

type builtinReceiver struct{} // NOTE: GrantMethod allows all access.

type methodWrapper struct {
	methodType                    int
	public                        bool
	fn                            reflect.Value
	requestType                   reflect.Type
	responseType                  reflect.Type
	failedCallsDistribution       *tricorder.CumulativeDistribution
	failedRRCallsDistribution     *tricorder.CumulativeDistribution
	numDeniedCalls                uint64
	numPermittedCalls             uint64
	successfulCallsDistribution   *tricorder.CumulativeDistribution
	successfulRRCallsDistribution *tricorder.CumulativeDistribution
}

type receiverType struct {
	methods     map[string]*methodWrapper
	blockMethod func(methodName string,
		authInfo *AuthInformation) (func(), error)
	grantMethod func(serviceMethod string, authInfo *AuthInformation) bool
}

var (
	defaultGrantMethod = func(serviceMethod string,
		authInfo *AuthInformation) bool {
		return false
	}
	receivers                    map[string]receiverType = make(map[string]receiverType)
	serverMetricsDir             *tricorder.DirectorySpec
	bucketer                     *tricorder.Bucketer
	serverMetricsMutex           sync.Mutex
	numPanicedCalls              uint64
	numServerConnections         uint64
	numOpenServerConnections     uint64
	numRejectedServerConnections uint64
	registerBuiltin              sync.Once
	registerBuiltinError         error

	computeHostname sync.Once
	hostname        string
	hostnameError   error
)

// Precompute some reflect types. Can't use the types directly because Typeof
// takes an empty interface value. This is annoying.
var typeOfConn = reflect.TypeOf((**Conn)(nil)).Elem()
var typeOfDecoder = reflect.TypeOf((*Decoder)(nil)).Elem()
var typeOfEncoder = reflect.TypeOf((*Encoder)(nil)).Elem()
var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

func init() {
	http.HandleFunc(getHostnamePath, getHostnameHttpHandler)
	http.HandleFunc(rpcPath, gobUnsecuredHttpHandler)
	http.HandleFunc(tlsRpcPath, gobTlsHttpHandler)
	http.HandleFunc(jsonRpcPath, jsonUnsecuredHttpHandler)
	http.HandleFunc(jsonTlsRpcPath, jsonTlsHttpHandler)
	http.HandleFunc(listMethodsPath, listMethodsHttpHandler)
	http.HandleFunc(listPublicMethodsPath, listPublicMethodsHttpHandler)
	registerServerMetrics()
}

func getNumPanicedCalls() uint64 {
	serverMetricsMutex.Lock()
	defer serverMetricsMutex.Unlock()
	return numPanicedCalls
}

func registerServerMetrics() {
	var err error
	serverMetricsDir, err = tricorder.RegisterDirectory("srpc/server")
	if err != nil {
		panic(err)
	}
	err = serverMetricsDir.RegisterMetric("num-connections",
		&numServerConnections, units.None, "number of connection attempts")
	if err != nil {
		panic(err)
	}
	err = serverMetricsDir.RegisterMetric("num-open-connections",
		&numOpenServerConnections, units.None, "number of open connections")
	if err != nil {
		panic(err)
	}
	err = serverMetricsDir.RegisterMetric("num-rejected-connections",
		&numRejectedServerConnections, units.None,
		"number of rejected connections")
	if err != nil {
		panic(err)
	}
	bucketer = tricorder.NewGeometricBucketer(0.1, 1e5)
}

func defaultMethodBlocker(methodName string,
	authInfo *AuthInformation) (func(), error) {
	return nil, nil
}

func defaultMethodGranter(serviceMethod string,
	authInfo *AuthInformation) bool {
	return defaultGrantMethod(serviceMethod, authInfo)
}

func registerName(name string, rcvr interface{},
	options ReceiverOptions) error {
	registerBuiltin.Do(func() {
		registerBuiltinError = _registerName("", &builtinReceiver{},
			ReceiverOptions{})
	})
	if registerBuiltinError != nil {
		return registerBuiltinError
	}
	return _registerName(name, rcvr, options)
}

func _registerName(name string, rcvr interface{},
	options ReceiverOptions) error {
	if _, ok := receivers[name]; ok {
		return fmt.Errorf("SRPC receiver already registered: %s", name)
	}
	receiver := receiverType{methods: make(map[string]*methodWrapper)}
	typeOfReceiver := reflect.TypeOf(rcvr)
	valueOfReceiver := reflect.ValueOf(rcvr)
	receiverMetricsDir, err := serverMetricsDir.RegisterDirectory(name)
	if err != nil {
		return err
	}
	publicMethods := stringutil.ConvertListToMap(options.PublicMethods, false)
	for index := 0; index < typeOfReceiver.NumMethod(); index++ {
		method := typeOfReceiver.Method(index)
		if method.PkgPath != "" { // Method must be exported.
			continue
		}
		methodType := method.Type
		mVal := getMethod(methodType, valueOfReceiver.Method(index))
		if mVal == nil {
			continue
		}
		receiver.methods[method.Name] = mVal
		if _, ok := publicMethods[method.Name]; ok {
			mVal.public = true
		}
		dir, err := receiverMetricsDir.RegisterDirectory(method.Name)
		if err != nil {
			return err
		}
		if err := mVal.registerMetrics(dir); err != nil {
			return err
		}
	}
	if blocker, ok := rcvr.(MethodBlocker); ok {
		receiver.blockMethod = blocker.BlockMethod
	} else {
		receiver.blockMethod = defaultMethodBlocker
	}
	if granter, ok := rcvr.(MethodGranter); ok {
		receiver.grantMethod = granter.GrantMethod
	} else {
		receiver.grantMethod = defaultMethodGranter
	}
	receivers[name] = receiver
	startReadingSmallStackMetaData()
	return nil
}

func (*builtinReceiver) GrantMethod(
	serviceMethod string, authInfo *AuthInformation) bool {
	return true
}

func getMethod(methodType reflect.Type, fn reflect.Value) *methodWrapper {
	if methodType.NumOut() != 1 {
		return nil
	}
	if methodType.Out(0) != typeOfError {
		return nil
	}
	if methodType.NumIn() == 2 {
		// Method needs two ins: receiver, *Conn.
		if methodType.In(1) != typeOfConn {
			return nil
		}
		return &methodWrapper{methodType: methodTypeRaw, fn: fn}
	}
	if methodType.NumIn() == 4 {
		if methodType.In(1) != typeOfConn {
			return nil
		}
		// Coder Method needs four ins: receiver, *Conn, Decoder, Encoder.
		if methodType.In(2) == typeOfDecoder &&
			methodType.In(3) == typeOfEncoder {
			return &methodWrapper{
				methodType: methodTypeCoder,
				fn:         fn,
			}
		}
		// RequestReply Method needs four ins: receiver, *Conn, request, *reply.
		if methodType.In(3).Kind() == reflect.Ptr {
			return &methodWrapper{
				methodType:   methodTypeRequestReply,
				fn:           fn,
				requestType:  methodType.In(2),
				responseType: methodType.In(3).Elem(),
			}
		}
	}
	return nil
}

func (m *methodWrapper) registerMetrics(dir *tricorder.DirectorySpec) error {
	m.failedCallsDistribution = bucketer.NewCumulativeDistribution()
	err := dir.RegisterMetric("failed-call-durations",
		m.failedCallsDistribution, units.Millisecond,
		"duration of failed calls")
	if err != nil {
		return err
	}
	err = dir.RegisterMetric("num-denied-calls", &m.numDeniedCalls,
		units.None, "number of denied calls to method")
	if err != nil {
		return err
	}
	err = dir.RegisterMetric("num-permitted-calls", &m.numPermittedCalls,
		units.None, "number of permitted calls to method")
	if err != nil {
		return err
	}
	m.successfulCallsDistribution = bucketer.NewCumulativeDistribution()
	err = dir.RegisterMetric("successful-call-durations",
		m.successfulCallsDistribution, units.Millisecond,
		"duration of successful calls")
	if err != nil {
		return err
	}
	if m.methodType != methodTypeRequestReply {
		return nil
	}
	m.failedRRCallsDistribution = bucketer.NewCumulativeDistribution()
	err = dir.RegisterMetric("failed-request-reply-call-durations",
		m.failedRRCallsDistribution, units.Millisecond,
		"duration of failed request-reply calls")
	if err != nil {
		return err
	}
	m.successfulRRCallsDistribution = bucketer.NewCumulativeDistribution()
	err = dir.RegisterMetric("successful-request-reply-call-durations",
		m.successfulRRCallsDistribution, units.Millisecond,
		"duration of successful request-reply calls")
	if err != nil {
		return err
	}
	return nil
}

func getHostnameHttpHandler(w http.ResponseWriter, req *http.Request) {
	computeHostname.Do(func() {
		cmd := exec.Command("hostname", "-f")
		output, err := cmd.CombinedOutput()
		if err == nil {
			hostname = strings.TrimSpace(string(output))
			return
		}
		hostname, hostnameError = os.Hostname()
	})
	if hostnameError != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, hostnameError)
		return
	}
	fmt.Fprintln(w, hostname)
}

func gobTlsHttpHandler(w http.ResponseWriter, req *http.Request) {
	httpHandler(w, req, true, &gobCoder{})
}

func gobUnsecuredHttpHandler(w http.ResponseWriter, req *http.Request) {
	httpHandler(w, req, false, &gobCoder{})
}

func jsonTlsHttpHandler(w http.ResponseWriter, req *http.Request) {
	httpHandler(w, req, true, &jsonCoder{})
}

func jsonUnsecuredHttpHandler(w http.ResponseWriter, req *http.Request) {
	httpHandler(w, req, false, &jsonCoder{})
}

func httpHandler(w http.ResponseWriter, req *http.Request, doTls bool,
	makeCoder coderMaker) {
	serverMetricsMutex.Lock()
	numServerConnections++
	serverMetricsMutex.Unlock()
	if doTls && serverTlsConfig == nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if (tlsRequired && !doTls) || req.Method != "CONNECT" {
		serverMetricsMutex.Lock()
		numRejectedServerConnections++
		serverMetricsMutex.Unlock()
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if tlsRequired && req.TLS != nil {
		if serverTlsConfig == nil ||
			!checkVerifiedChains(req.TLS.VerifiedChains,
				serverTlsConfig.ClientCAs) {
			serverMetricsMutex.Lock()
			numRejectedServerConnections++
			serverMetricsMutex.Unlock()
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}
	if req.ParseForm() != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		logger.Println("not a hijacker ", req.RemoteAddr)
		return
	}
	unsecuredConn, bufrw, err := hijacker.Hijack()
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		logger.Println("rpc hijacking ", req.RemoteAddr, ": ", err.Error())
		return
	}
	myConn := &Conn{
		conn:       unsecuredConn,
		localAddr:  unsecuredConn.LocalAddr().String(),
		remoteAddr: unsecuredConn.RemoteAddr().String(),
	}
	connType := "unknown"
	defer func() {
		myConn.conn.Close()
	}()
	if tcpConn, ok := unsecuredConn.(net.TCPConn); ok {
		connType = "TCP"
		if err := tcpConn.SetKeepAlive(true); err != nil {
			logger.Println("error setting keepalive: ", err.Error())
			return
		}
		if err := tcpConn.SetKeepAlivePeriod(time.Minute * 5); err != nil {
			logger.Println("error setting keepalive period: ", err.Error())
			return
		}
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotAcceptable)
		logger.Println("non-TCP connection")
		return
	}
	_, err = io.WriteString(unsecuredConn, "HTTP/1.0 "+connectString+"\n\n")
	if err != nil {
		logger.Println("error writing connect message: ", err.Error())
		return
	}
	allowMethodPowers := true
	if req.Form.Get(doNotUseMethodPowers) == "true" {
		allowMethodPowers = false
	}
	if doTls {
		var tlsConn *tls.Conn
		if req.TLS == nil {
			tlsConn = tls.Server(unsecuredConn, serverTlsConfig)
			myConn.conn = tlsConn
			if err := tlsConn.Handshake(); err != nil {
				serverMetricsMutex.Lock()
				numRejectedServerConnections++
				serverMetricsMutex.Unlock()
				logger.Println(err)
				return
			}
			connType += "/TLS"
		} else {
			if tlsConn, ok = unsecuredConn.(*tls.Conn); !ok {
				logger.Println("not really a TLS connection")
				return
			}
			connType += "/TLS"
		}
		myConn.isEncrypted = true
		myConn.username, myConn.permittedMethods, myConn.groupList, err =
			getAuth(tlsConn.ConnectionState(), allowMethodPowers)
		if err != nil {
			logger.Println(err)
			return
		}
		myConn.ReadWriter = bufio.NewReadWriter(bufio.NewReader(tlsConn),
			bufio.NewWriter(tlsConn))
	} else {
		if !allowMethodPowers {
			myConn.permittedMethods = make(map[string]struct{})
		}
		myConn.ReadWriter = bufrw
	}
	logger.Debugf(0, "accepted %s connection from: %s\n",
		connType, myConn.remoteAddr)
	serverMetricsMutex.Lock()
	numOpenServerConnections++
	serverMetricsMutex.Unlock()
	handleConnection(myConn, makeCoder)
	serverMetricsMutex.Lock()
	numOpenServerConnections--
	serverMetricsMutex.Unlock()
}

func checkVerifiedChains(verifiedChains [][]*x509.Certificate,
	certPool *x509.CertPool) bool {
	for _, vChain := range verifiedChains {
		vSubject := vChain[0].RawIssuer
		for _, cSubject := range certPool.Subjects() {
			if bytes.Compare(vSubject, cSubject) == 0 {
				return true
			}
		}
	}
	return false
}

func getAuth(state tls.ConnectionState, allowMethodPowers bool) (
	string, map[string]struct{},
	map[string]struct{}, error) {
	var username string
	permittedMethods := make(map[string]struct{})
	trustCertMethods := false
	if fullAuthCaCertPool == nil ||
		checkVerifiedChains(state.VerifiedChains, fullAuthCaCertPool) {
		trustCertMethods = true
	}
	var groupList map[string]struct{}
	for _, certChain := range state.VerifiedChains {
		for _, cert := range certChain {
			var err error
			if username == "" {
				username, err = x509util.GetUsername(cert)
				if err != nil {
					return "", nil, nil, err
				}
			}
			if len(groupList) < 1 {
				groupList, err = x509util.GetGroupList(cert)
				if err != nil {
					return "", nil, nil, err
				}
			}
			if allowMethodPowers && trustCertMethods {
				pms, err := x509util.GetPermittedMethods(cert)
				if err != nil {
					return "", nil, nil, err
				}
				for method := range pms {
					permittedMethods[method] = struct{}{}
				}
			}
		}
	}
	return username, permittedMethods, groupList, nil
}

func handleConnection(conn *Conn, makeCoder coderMaker) {
	defer conn.callReleaseNotifier()
	defer conn.Flush()
	for ; ; conn.Flush() {
		conn.callReleaseNotifier()
		serviceMethod, err := conn.ReadString('\n')
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return
		}
		if err != nil {
			logger.Println(err)
			if _, err := conn.WriteString(err.Error() + "\n"); err != nil {
				logger.Println(err)
				return
			}
			continue
		}
		serviceMethod = strings.TrimSpace(serviceMethod)
		if serviceMethod == "" {
			// Received a "ping" request, send response.
			if _, err := conn.WriteString("\n"); err != nil {
				logger.Println(err)
				return
			}
			continue
		}
		method, err := conn.findMethod(serviceMethod)
		if err != nil {
			if _, err := conn.WriteString(err.Error() + "\n"); err != nil {
				logger.Println(err)
				return
			}
			continue
		}
		// Method is OK to call. Tell client and then call method handler.
		if _, err := conn.WriteString("\n"); err != nil {
			logger.Println(err)
			return
		}
		if err := conn.Flush(); err != nil {
			logger.Println(err)
			return
		}
		if err := method.call(conn, makeCoder); err != nil {
			if err != ErrorCloseClient {
				logger.Println(err)
			}
			return
		}
	}
}

func (conn *Conn) callReleaseNotifier() {
	if releaseNotifier := conn.releaseNotifier; releaseNotifier != nil {
		releaseNotifier()
	}
	conn.releaseNotifier = nil
}

func (conn *Conn) findMethod(serviceMethod string) (*methodWrapper, error) {
	splitServiceMethod := strings.Split(serviceMethod, ".")
	if len(splitServiceMethod) != 2 {
		return nil, errors.New("malformed Service.Method: " + serviceMethod)
	}
	serviceName := splitServiceMethod[0]
	receiver, ok := receivers[serviceName]
	if !ok {
		return nil, errors.New("unknown service: " + serviceName)
	}
	methodName := splitServiceMethod[1]
	method, ok := receiver.methods[methodName]
	if !ok {
		return nil, errors.New(serviceName + ": unknown method: " + methodName)
	}
	if conn.checkMethodAccess(serviceMethod) {
		conn.haveMethodAccess = true
	} else if receiver.grantMethod(serviceName, conn.GetAuthInformation()) {
		conn.haveMethodAccess = true
	} else {
		conn.haveMethodAccess = false
		if !method.public {
			method.numDeniedCalls++
			return nil, ErrorAccessToMethodDenied
		}
	}
	authInfo := conn.GetAuthInformation()
	if rn, err := receiver.blockMethod(methodName, authInfo); err != nil {
		return nil, err
	} else {
		conn.releaseNotifier = rn
	}
	return method, nil
}

// checkMethodAccess implements the built-in authorisation checks. It returns
// true if the method is permitted, else false if denied.
func (conn *Conn) checkMethodAccess(serviceMethod string) bool {
	if conn.permittedMethods == nil {
		return true
	}
	for sm := range conn.permittedMethods {
		if matched, _ := filepath.Match(sm, serviceMethod); matched {
			return true
		}
	}
	if conn.username != "" {
		smallStackOwners := getSmallStackOwners()
		if smallStackOwners != nil {
			if _, ok := smallStackOwners.users[conn.username]; ok {
				return true
			}
			for _, group := range smallStackOwners.groups {
				if _, ok := conn.groupList[group]; ok {
					return true
				}
			}
		}
	}
	return false
}

func listMethodsHttpHandler(w http.ResponseWriter, req *http.Request) {
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	methods := make([]string, len(receivers))
	for receiverName, receiver := range receivers {
		for method := range receiver.methods {
			methods = append(methods, receiverName+"."+method+"\n")
		}
	}
	sort.Strings(methods)
	for _, method := range methods {
		writer.WriteString(method)
	}
}

func listPublicMethodsHttpHandler(w http.ResponseWriter, req *http.Request) {
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	methods := make([]string, len(receivers))
	for receiverName, receiver := range receivers {
		for name, method := range receiver.methods {
			if !method.public {
				continue
			}
			methods = append(methods, receiverName+"."+name+"\n")
		}
	}
	sort.Strings(methods)
	for _, method := range methods {
		writer.WriteString(method)
	}
}

func (m *methodWrapper) call(conn *Conn, makeCoder coderMaker) error {
	m.numPermittedCalls++
	startTime := time.Now()
	err := m._call(conn, makeCoder)
	timeTaken := time.Since(startTime)
	if err == nil {
		m.successfulCallsDistribution.Add(timeTaken)
	} else {
		m.failedCallsDistribution.Add(timeTaken)
	}
	return err
}

func (m *methodWrapper) _call(conn *Conn, makeCoder coderMaker) error {
	defer func() {
		if err := recover(); err != nil {
			serverMetricsMutex.Lock()
			numPanicedCalls++
			serverMetricsMutex.Unlock()
			panic(err)
		}
	}()
	connValue := reflect.ValueOf(conn)
	conn.Decoder = makeCoder.MakeDecoder(conn)
	conn.Encoder = makeCoder.MakeEncoder(conn)
	switch m.methodType {
	case methodTypeRaw:
		returnValues := m.fn.Call([]reflect.Value{connValue})
		errInter := returnValues[0].Interface()
		if errInter != nil {
			return errInter.(error)
		}
		return nil
	case methodTypeCoder:
		returnValues := m.fn.Call([]reflect.Value{
			connValue,
			reflect.ValueOf(conn.Decoder),
			reflect.ValueOf(conn.Encoder),
		})
		errInter := returnValues[0].Interface()
		if errInter != nil {
			return errInter.(error)
		}
		return nil
	case methodTypeRequestReply:
		request := reflect.New(m.requestType)
		response := reflect.New(m.responseType)
		if err := conn.Decode(request.Interface()); err != nil {
			_, err = conn.WriteString(err.Error() + "\n")
			return err
		}
		startTime := time.Now()
		returnValues := m.fn.Call([]reflect.Value{connValue, request.Elem(),
			response})
		timeTaken := time.Since(startTime)
		errInter := returnValues[0].Interface()
		if errInter != nil {
			m.failedRRCallsDistribution.Add(timeTaken)
			err := errInter.(error)
			_, err = conn.WriteString(err.Error() + "\n")
			return err
		}
		m.successfulRRCallsDistribution.Add(timeTaken)
		if _, err := conn.WriteString("\n"); err != nil {
			return err
		}
		return conn.Encode(response.Interface())
	}
	return errors.New("unknown method type")
}
