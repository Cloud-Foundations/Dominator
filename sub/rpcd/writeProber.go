package rpcd

import (
	"fmt"
	"os"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
)

func (t *rpcType) startWriteProber() {
	if t.params.SubdDirectory == "" {
		return
	}
	var counter uint64
	for {
		errString := errors.ErrorToString(t.writeProbe(counter))
		t.rwLock.Lock()
		t.lastWriteError = errString
		t.rwLock.Unlock()
		counter++
		time.Sleep(5 * time.Minute)
	}
}

func (t *rpcType) writeProbe(counter uint64) (err error) {
	filename := fmt.Sprintf("%s/write-probe.%d",
		t.params.SubdDirectory, counter)
	file, err := os.Create(filename)
	if err != nil {
		return
	}
	defer func() {
		if e := os.Remove(filename); err == nil && e != nil {
			err = e
		}
	}()
	data := []byte(fmt.Sprintf("data.%d\n", counter))
	if _, err = file.Write(data); err != nil {
		return
	}
	if err = file.Sync(); err != nil {
		return
	}
	return
}
