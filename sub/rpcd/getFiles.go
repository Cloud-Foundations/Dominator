package rpcd

import (
	"bufio"
	"io"
	"os"
	"path"

	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/sub"
)

func (t *rpcType) GetFiles(conn *srpc.Conn) error {
	defer conn.Flush()
	t.getFilesLock.Lock()
	defer t.getFilesLock.Unlock()
	numFiles := 0
	for ; ; numFiles++ {
		filename, err := conn.ReadString('\n')
		if err != nil {
			return err
		}
		filename = filename[:len(filename)-1]
		if filename == "" {
			break
		}
		filename = path.Join(t.config.RootDirectoryName, filename)
		if err := t.getFile(conn, filename); err != nil {
			return err
		}
	}
	plural := "s"
	if numFiles == 1 {
		plural = ""
	}
	t.params.Logger.Printf("GetFiles(): %d file%s provided\n", numFiles, plural)
	return nil
}

func (t *rpcType) getFile(conn *srpc.Conn, filename string) error {
	var file *os.File
	var err error
	t.params.WorkdirGoroutine.Run(func() { file, err = os.Open(filename) })
	var response sub.GetFileResponse
	if err != nil {
		response.Error = err.Error()
	} else {
		defer file.Close()
		if fi, err := file.Stat(); err != nil {
			response.Error = err.Error()
		} else {
			response.Size = uint64(fi.Size())
		}
	}
	if err := conn.Encode(response); err != nil {
		return err
	}
	if response.Error != "" {
		return nil
	}
	_, err = io.Copy(conn, bufio.NewReader(file))
	return err
}
