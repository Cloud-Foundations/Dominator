//go:build linux
// +build linux

package main

import (
	"bufio"
	"fmt"
	"io"
	stdlog "log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"sync"
	"syscall"

	"github.com/Cloud-Foundations/Dominator/lib/html"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/debuglogger"
	"github.com/Cloud-Foundations/Dominator/lib/log/teelogger"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/verstr"
)

type HtmlWriter interface {
	WriteHtml(writer io.Writer)
}

type sessionType struct {
	device   string
	username string
}

type srpcType struct {
	remoteShellWaitGroup *sync.WaitGroup
	logger               log.DebugLogger
	mutex                sync.RWMutex
	connections          map[*srpc.Conn]sessionType
}

type state struct {
	logger  log.DebugLogger
	srpcObj *srpcType
}

var htmlWriters []HtmlWriter

func copyFromPty(conn *srpc.Conn, pty io.Reader, killed *bool,
	logger log.Logger) {
	buffer := make([]byte, 256)
	for {
		if nRead, err := pty.Read(buffer); err != nil {
			if *killed {
				break
			}
			logger.Printf("error reading from pty: %s", err)
			break
		} else if _, err := conn.Write(buffer[:nRead]); err != nil {
			logger.Printf("error writing to connection: %s\n", err)
			break
		}
		if err := conn.Flush(); err != nil {
			logger.Printf("error flushing connection: %s\n", err)
			break
		}
	}
}

func copyToPty(pty io.Writer, reader io.Reader) error {
	buffer := make([]byte, 256)
	for {
		if nRead, err := reader.Read(buffer); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		} else {
			if _, err := pty.Write(buffer[:nRead]); err != nil {
				return fmt.Errorf("error writing to pty: %w", err)
			}
		}
	}
}

func startServer(portNum uint, remoteShellWaitGroup *sync.WaitGroup,
	logger log.DebugLogger) (log.DebugLogger, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", portNum))
	if err != nil {
		return nil, err
	}
	srpcObj := &srpcType{
		remoteShellWaitGroup: remoteShellWaitGroup,
		logger:               logger,
		connections:          make(map[*srpc.Conn]sessionType),
	}
	myState := state{logger, srpcObj}
	html.HandleFunc("/", myState.statusHandler)
	if err := srpc.RegisterName("Installer", srpcObj); err != nil {
		logger.Printf("error registering SRPC receiver: %s\n", err)
	}
	sprayLogger := debuglogger.New(stdlog.New(&logWriter{srpcObj}, "", 0))
	sprayLogger.SetLevel(int16(*logDebugLevel))
	go http.Serve(listener, nil)
	return teelogger.New(logger, sprayLogger), nil
}

func (s state) statusHandler(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		http.NotFound(w, req)
		return
	}
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	fmt.Fprintln(writer, "<title>installer status page</title>")
	fmt.Fprintln(writer, `<style>
                          table, th, td {
                          border-collapse: collapse;
                          }
                          </style>`)
	fmt.Fprintln(writer, "<body>")
	fmt.Fprintln(writer, "<center>")
	fmt.Fprintln(writer, "<h1>installer status page</h1>")
	fmt.Fprintln(writer, "</center>")
	html.WriteHeaderWithRequest(writer, req)
	fmt.Fprintln(writer, "<h3>")
	s.writeDashboard(writer)
	for _, htmlWriter := range htmlWriters {
		htmlWriter.WriteHtml(writer)
	}
	fmt.Fprintln(writer, "</h3>")
	fmt.Fprintln(writer, "<hr>")
	html.WriteFooter(writer)
	fmt.Fprintln(writer, "</body>")
}

func AddHtmlWriter(htmlWriter HtmlWriter) {
	htmlWriters = append(htmlWriters, htmlWriter)
}

func (s state) writeDashboard(writer io.Writer) {
	var sessions []sessionType
	s.srpcObj.mutex.RLock()
	for _, session := range s.srpcObj.connections {
		sessions = append(sessions, session)
	}
	s.srpcObj.mutex.RUnlock()
	if len(sessions) < 1 {
		return
	}
	sort.SliceStable(sessions, func(left, right int) bool {
		return verstr.Less(sessions[left].device, sessions[right].device)
	})
	fmt.Fprintln(writer, "Login sessions:<br>")
	fmt.Fprintln(writer, `<table border="1">`)
	tw, _ := html.NewTableWriter(writer, true, "Terminal", "Username")
	for _, session := range sessions {
		tw.WriteRow("", "", session.device, session.username)
	}
	tw.Close()
}

func (t *srpcType) Shell(conn *srpc.Conn) error {
	t.remoteShellWaitGroup.Add(1)
	defer t.remoteShellWaitGroup.Done()
	pty, tty, err := openPty()
	if err != nil {
		return err
	}
	defer pty.Close()
	defer tty.Close()
	session := sessionType{
		device:   tty.Name(),
		username: conn.Username(),
	}
	t.logger.Printf(
		"shell on SRPC connection started for user: %s with tty: %s\n",
		session.username, session.device)
	if file, err := os.Open("/var/log/installer/latest"); err != nil {
		t.logger.Println(err)
	} else {
		fmt.Fprintln(conn, "Logs so far:\r")
		// Need to inject carriage returns for each line, so have to do this the
		// hard way.
		reader := bufio.NewReader(file)
		for {
			if chunk, isPrefix, err := reader.ReadLine(); err != nil {
				break
			} else {
				conn.Write(chunk)
				if !isPrefix {
					conn.Write([]byte("\r\n"))
				}
			}
		}
		file.Close()
		conn.Flush()
	}
	// Begin sending new logs back.
	t.mutex.Lock()
	t.connections[conn] = session
	t.mutex.Unlock()
	cmd := exec.Command("/bin/busybox", "sh", "-i")
	cmd.Env = make([]string, 0)
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
	if err := cmd.Start(); err != nil {
		t.mutex.Lock()
		delete(t.connections, conn)
		t.mutex.Unlock()
		return err
	}
	fmt.Fprintf(conn, "Starting shell on: %s...\r\n", tty.Name())
	conn.Flush()
	killed := false
	go func() { // Read from pty until killed.
		copyFromPty(conn, pty, &killed, t.logger)
		t.mutex.Lock()
		delete(t.connections, conn)
		t.mutex.Unlock()
	}()
	// Read from connection, write to pty.
	err = copyToPty(pty, conn)
	killed = true
	cmd.Process.Kill()
	cmd.Wait()
	if err == nil {
		t.logger.Printf(
			"shell on SRPC connection exited for user: %s with tty: %s\n",
			session.username, session.device)
	}
	return err
}

func (t *srpcType) Write(p []byte) (int, error) {
	buffer := make([]byte, 0, len(p)+1)
	for _, ch := range p { // First add a carriage return for each newline.
		if ch == '\n' {
			buffer = append(buffer, '\r')
		}
		buffer = append(buffer, ch)
	}
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	for conn := range t.connections {
		conn.Write(buffer)
		conn.Flush()
	}
	return len(p), nil
}
