package serverlogger

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	liblog "github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/logbuf"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/srpc/serverutil"
	proto "github.com/Cloud-Foundations/Dominator/proto/logger"
)

type loggerMapT struct {
	*serverutil.PerUserMethodLimiter
	sync.Mutex
	loggerMap map[string]*Logger
}

type grabWriter struct {
	data []byte
}

var loggerMap *loggerMapT = &loggerMapT{
	loggerMap: make(map[string]*Logger),
	PerUserMethodLimiter: serverutil.NewPerUserMethodLimiter(
		map[string]uint{
			"Debug":         1,
			"Print":         1,
			"SetDebugLevel": 1,
			"Watch":         1,
		}),
}

func init() {
	srpc.RegisterName("Logger", loggerMap)
}

func getCallerName(depth int) string {
	if pc, _, _, ok := runtime.Caller(depth); !ok {
		return "UNKNOWN"
	} else if fi := runtime.FuncForPC(pc); fi == nil {
		return "UNKNOWN"
	} else if splitName := strings.Split(fi.Name(), "."); len(splitName) < 1 {
		return "UNKNOWN"
	} else {
		return splitName[len(splitName)-1]
	}
}

func (w *grabWriter) Write(p []byte) (int, error) {
	w.data = p
	return len(p), nil
}

func newLogger(name string, options logbuf.Options, flags int) *Logger {
	if name == "" && !options.AlsoLogToStderr {
		options.RedirectStderr = true
	}
	loggerMap.Lock()
	defer loggerMap.Unlock()
	if _, ok := loggerMap.loggerMap[name]; ok {
		panic("logger already exists: " + name)
	}
	circularBuffer := logbuf.NewWithOptions(options)
	logger := &Logger{
		circularBuffer: circularBuffer,
		flags:          flags,
		level:          int16(*initialLogDebugLevel),
		streamers:      make(map[*streamerType]struct{}),
	}
	if logger.level < -1 {
		logger.level = -1
	}
	logger.maxLevel = logger.level
	// Ensure this satisfies the published interface.
	var debugLogger liblog.FullDebugLogger
	debugLogger = logger
	_ = debugLogger
	loggerMap.loggerMap[name] = logger
	if *logAtStartup {
		logger.Println("Startup")
	}
	return logger
}

func (l *Logger) checkAuth(authInfo *srpc.AuthInformation) error {
	if authInfo.HaveMethodAccess {
		return nil
	}
	if accessChecker := l.accessChecker; accessChecker == nil {
		return errors.New("no access to resource")
	} else if accessChecker(getCallerName(3), authInfo) {
		return nil
	} else {
		return errors.New("no access to resource")
	}
}

func (l *Logger) debug(level int16, v ...interface{}) {
	if l.maxLevel >= level {
		l.log(level, fmt.Sprint(v...), false)
	}
}

func (l *Logger) debugf(level int16, format string, v ...interface{}) {
	if l.maxLevel >= level {
		l.log(level, fmt.Sprintf(format, v...), false)
	}
}

func (l *Logger) debugln(level int16, v ...interface{}) {
	if l.maxLevel >= level {
		l.log(level, fmt.Sprintln(v...), false)
	}
}

func (l *Logger) fatals(msg string) {
	l.log(-1, msg, true)
	os.Exit(1)
}

func (l *Logger) log(level int16, msg string, dying bool) {
	buffer := &grabWriter{}
	rawLogger := log.New(buffer, "", l.flags)
	rawLogger.Output(4, msg)
	if l.level >= level {
		l.circularBuffer.Write(buffer.data)
	}
	recalculateLevels := false
	l.mutex.Lock()
	defer l.mutex.Unlock()
	for streamer := range l.streamers {
		if streamer.debugLevel >= level &&
			(streamer.includeRegex == nil ||
				streamer.includeRegex.Match(buffer.data)) &&
			(streamer.excludeRegex == nil ||
				!streamer.excludeRegex.Match(buffer.data)) {
			select {
			case streamer.output <- buffer.data:
			default:
				delete(l.streamers, streamer)
				close(streamer.output)
				recalculateLevels = true
			}
		}
	}
	if dying {
		for streamer := range l.streamers {
			delete(l.streamers, streamer)
			close(streamer.output)
		}
		l.circularBuffer.Flush()
		time.Sleep(time.Millisecond * 10)
	} else if recalculateLevels {
		l.updateMaxLevel()
	}
}

func (l *Logger) makeStreamer(request proto.WatchRequest) (
	*streamerType, error) {
	if request.DebugLevel < -1 {
		request.DebugLevel = -1
	}
	streamer := &streamerType{debugLevel: request.DebugLevel}
	if request.ExcludeRegex != "" {
		var err error
		streamer.excludeRegex, err = regexp.Compile(request.ExcludeRegex)
		if err != nil {
			return nil, err
		}
	}
	if request.IncludeRegex != "" {
		var err error
		streamer.includeRegex, err = regexp.Compile(request.IncludeRegex)
		if err != nil {
			return nil, err
		}
	}
	return streamer, nil
}

func (l *Logger) panics(msg string) {
	l.log(-1, msg, true)
	panic(msg)
}

func (l *Logger) prints(msg string) {
	l.log(-1, msg, false)
}

func (l *Logger) setLevel(maxLevel int16) {
	if maxLevel < -1 {
		maxLevel = -1
	}
	l.level = maxLevel
	l.mutex.Lock()
	l.updateMaxLevel()
	l.mutex.Unlock()
}

func (l *Logger) updateMaxLevel() {
	maxLevel := l.level
	for streamer := range l.streamers {
		if streamer.debugLevel > maxLevel {
			maxLevel = streamer.debugLevel
		}
	}
	l.maxLevel = maxLevel
}

func (l *Logger) watch(conn *srpc.Conn, streamer *streamerType) {
	channel := make(chan []byte, 256)
	streamer.output = channel
	l.mutex.Lock()
	l.streamers[streamer] = struct{}{}
	l.updateMaxLevel()
	l.mutex.Unlock()
	timer := time.NewTimer(time.Millisecond * 100)
	flushPending := false
	closeNotifier := conn.GetCloseNotifier()
	for keepGoing := true; keepGoing; {
		select {
		case <-closeNotifier:
			keepGoing = false
			break
		case data, ok := <-channel:
			if !ok {
				keepGoing = false
				break
			}
			if _, err := conn.Write(data); err != nil {
				keepGoing = false
				break
			}
			if !flushPending {
				timer.Reset(time.Millisecond * 100)
				flushPending = true
			}
		case <-timer.C:
			if conn.Flush() != nil {
				keepGoing = false
				break
			}
			flushPending = false
		}
	}
	if flushPending {
		conn.Flush()
	}
	l.mutex.Lock()
	delete(l.streamers, streamer)
	l.updateMaxLevel()
	l.mutex.Unlock()
	// Drain the channel.
	for {
		select {
		case <-channel:
		default:
			return
		}
	}
}

func (t *loggerMapT) getLogger(name string,
	authInfo *srpc.AuthInformation) (*Logger, error) {
	loggerMap.Lock()
	defer loggerMap.Unlock()
	if logger, ok := loggerMap.loggerMap[name]; !ok {
		return nil, errors.New("unknown logger: " + name)
	} else if err := logger.checkAuth(authInfo); err != nil {
		return nil, err
	} else {
		return logger, nil
	}
}

func (t *loggerMapT) Debug(conn *srpc.Conn,
	request proto.DebugRequest, reply *proto.DebugResponse) error {
	authInfo := conn.GetAuthInformation()
	if logger, err := t.getLogger(request.Name, authInfo); err != nil {
		return err
	} else {
		logger.Debugf(request.Level, "Logger.Debug(%d): %s\n",
			request.Level, strings.Join(request.Args, " "))
		return nil
	}
}

func (t *loggerMapT) Print(conn *srpc.Conn,
	request proto.PrintRequest,
	reply *proto.PrintResponse) error {
	authInfo := conn.GetAuthInformation()
	if logger, err := t.getLogger(request.Name, authInfo); err != nil {
		return err
	} else {
		logger.Println("Logger.Print():", strings.Join(request.Args, " "))
		return nil
	}
}

func (t *loggerMapT) SetDebugLevel(conn *srpc.Conn,
	request proto.SetDebugLevelRequest,
	reply *proto.SetDebugLevelResponse) error {
	authInfo := conn.GetAuthInformation()
	if logger, err := t.getLogger(request.Name, authInfo); err != nil {
		return err
	} else {
		logger.Printf("Logger.SetDebugLevel(%d)\n", request.Level)
		logger.SetLevel(request.Level)
		return nil
	}
}

func (t *loggerMapT) Watch(conn *srpc.Conn) error {
	var request proto.WatchRequest
	if err := conn.Decode(&request); err != nil {
		return err
	}
	authInfo := conn.GetAuthInformation()
	if logger, err := t.getLogger(request.Name, authInfo); err != nil {
		return conn.Encode(proto.WatchResponse{Error: err.Error()})
	} else if streamer, err := logger.makeStreamer(request); err != nil {
		return conn.Encode(proto.WatchResponse{Error: err.Error()})
	} else {
		if err := conn.Encode(proto.WatchResponse{}); err != nil {
			return err
		}
		if err := conn.Flush(); err != nil {
			return err
		}
		logger.watch(conn, streamer)
		return srpc.ErrorCloseClient
	}
}

func (l *Logger) writeHtml(writer io.Writer) {
	if numPanics := srpc.GetNumPanicedCalls(); numPanics > 0 {
		var suffix string
		if numPanics > 1 {
			suffix = "s"
		}
		fmt.Fprintf(writer,
			"<p><font color=\"red\">SRPC Server recorded %d call%s which paniced</font><br>",
			numPanics, suffix)
	}
	l.circularBuffer.WriteHtml(writer)
}
