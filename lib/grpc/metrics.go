package grpc

import (
	"sync"
	"time"

	"github.com/Cloud-Foundations/tricorder/go/tricorder"
	"github.com/Cloud-Foundations/tricorder/go/tricorder/units"
)

type methodMetrics struct {
	numDeniedCalls              uint64
	numPermittedCalls           uint64
	failedCallsDistribution     *tricorder.CumulativeDistribution
	successfulCallsDistribution *tricorder.CumulativeDistribution
}

var (
	metricsMutex         sync.Mutex
	grpcMetricsDir       *tricorder.DirectorySpec
	restMetricsDir       *tricorder.DirectorySpec
	bucketer             *tricorder.Bucketer
	grpcMethodMetricsMap = make(map[string]*methodMetrics)
	restMethodMetricsMap = make(map[string]*methodMetrics)

	// gRPC metrics
	numConnections     uint64
	numOpenConnections uint64
	numRunningMethods  uint64
	numPanicedCalls    uint64

	// Rest metrics
	numRestRunningMethods uint64
)

func init() {
	initServerMetrics()
}

func initServerMetrics() {
	var err error
	bucketer = tricorder.NewGeometricBucketer(0.1, 1e5)

	// gRPC metrics
	grpcMetricsDir, err = tricorder.RegisterDirectory("grpc/server")
	if err != nil {
		panic(err)
	}
	err = grpcMetricsDir.RegisterMetric("num-connections",
		&numConnections, units.None, "number of connection attempts")
	if err != nil {
		panic(err)
	}
	err = grpcMetricsDir.RegisterMetric("num-open-connections",
		&numOpenConnections, units.None, "number of open connections")
	if err != nil {
		panic(err)
	}
	err = grpcMetricsDir.RegisterMetric("num-running-methods",
		&numRunningMethods, units.None, "number of running methods")
	if err != nil {
		panic(err)
	}
	err = grpcMetricsDir.RegisterMetric("num-paniced-calls",
		&numPanicedCalls, units.None, "number of paniced calls")
	if err != nil {
		panic(err)
	}

	// Rest metrics
	restMetricsDir, err = tricorder.RegisterDirectory("rest/server")
	if err != nil {
		panic(err)
	}
	err = restMetricsDir.RegisterMetric("num-running-methods",
		&numRestRunningMethods, units.None, "number of running methods")
	if err != nil {
		panic(err)
	}
}

func registerMethodMetrics(serviceName string, methods map[string]struct{}) {
	for fullMethod := range methods {
		createMethodMetrics(grpcMetricsDir, grpcMethodMetricsMap, fullMethod)
		createMethodMetrics(restMetricsDir, restMethodMetricsMap, fullMethod)
	}
}

func createMethodMetrics(metricsDir *tricorder.DirectorySpec,
	methodMap map[string]*methodMetrics, fullMethod string) *methodMetrics {

	metricsMutex.Lock()
	defer metricsMutex.Unlock()
	if m, ok := methodMap[fullMethod]; ok {
		return m
	}
	m := &methodMetrics{
		failedCallsDistribution:     bucketer.NewCumulativeDistribution(),
		successfulCallsDistribution: bucketer.NewCumulativeDistribution(),
	}
	methodPath := fullMethod
	if len(methodPath) > 0 && methodPath[0] == '/' {
		methodPath = methodPath[1:]
	}
	dir, err := metricsDir.RegisterDirectory(methodPath)
	if err != nil {
		methodMap[fullMethod] = m
		return m
	}

	dir.RegisterMetric("num-denied-calls", &m.numDeniedCalls,
		units.None, "number of denied calls to method")
	dir.RegisterMetric("num-permitted-calls", &m.numPermittedCalls,
		units.None, "number of permitted calls to method")
	dir.RegisterMetric("failed-call-durations", m.failedCallsDistribution,
		units.Millisecond, "duration of failed calls")
	dir.RegisterMetric("successful-call-durations", m.successfulCallsDistribution,
		units.Millisecond, "duration of successful calls")

	methodMap[fullMethod] = m
	return m
}

func getMethodMetrics(methodMap map[string]*methodMetrics, fullMethod string) *methodMetrics {
	metricsMutex.Lock()
	defer metricsMutex.Unlock()
	return methodMap[fullMethod]
}

// gRPC metrics functions

func recordDeniedCall(fullMethod string) {
	m := getMethodMetrics(grpcMethodMetricsMap, fullMethod)
	if m == nil {
		return
	}
	metricsMutex.Lock()
	m.numDeniedCalls++
	metricsMutex.Unlock()
}

func recordCallStart() {
	metricsMutex.Lock()
	numRunningMethods++
	metricsMutex.Unlock()
}

func recordCallEnd(fullMethod string, startTime time.Time, err error) {
	duration := time.Since(startTime)
	m := getMethodMetrics(grpcMethodMetricsMap, fullMethod)

	metricsMutex.Lock()
	numRunningMethods--
	if m != nil {
		m.numPermittedCalls++
	}
	metricsMutex.Unlock()

	if m != nil {
		if err != nil {
			m.failedCallsDistribution.Add(duration)
		} else {
			m.successfulCallsDistribution.Add(duration)
		}
	}
}

func recordPanic() {
	metricsMutex.Lock()
	numPanicedCalls++
	numRunningMethods--
	metricsMutex.Unlock()
}

// Rest metrics functions

func recordRestDeniedCall(fullMethod string) {
	m := getMethodMetrics(restMethodMetricsMap, fullMethod)
	if m == nil {
		return
	}
	metricsMutex.Lock()
	m.numDeniedCalls++
	metricsMutex.Unlock()
}

func recordRestCallStart() {
	metricsMutex.Lock()
	numRestRunningMethods++
	metricsMutex.Unlock()
}

func recordRestCallEnd(fullMethod string, startTime time.Time, err error) {
	duration := time.Since(startTime)
	m := getMethodMetrics(restMethodMetricsMap, fullMethod)

	metricsMutex.Lock()
	numRestRunningMethods--
	if m != nil {
		m.numPermittedCalls++
	}
	metricsMutex.Unlock()

	if m != nil {
		if err != nil {
			m.failedCallsDistribution.Add(duration)
		} else {
			m.successfulCallsDistribution.Add(duration)
		}
	}
}
