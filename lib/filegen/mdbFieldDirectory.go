package filegen

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
)

var (
	machineType = reflect.TypeOf(mdb.Machine{})
)

type mdbFieldDirectoryType struct {
	directory string
	index     []int
}

func sendNotifications(notifierChannel chan<- string, interval time.Duration) {
	for ; ; time.Sleep(interval) {
		notifierChannel <- ""
	}
}

func (m *Manager) registerMdbFieldDirectoryForPath(pathname string,
	field, directory string, interval time.Duration) error {
	structField, found := machineType.FieldByName(field)
	if !found {
		return fmt.Errorf("field: \"%s\" not found in mdb.Machine type", field)
	}
	if structField.Type.Kind() != reflect.String {
		return fmt.Errorf("field: \"%s\" is not string type", field)
	}
	generator := &mdbFieldDirectoryType{
		directory: directory,
		index:     structField.Index,
	}
	notifierChannel := m.RegisterGeneratorForPath(pathname, generator)
	if interval <= 0 {
		close(notifierChannel)
		return nil
	}
	go sendNotifications(notifierChannel, interval)
	return nil
}

func (g *mdbFieldDirectoryType) Generate(machine mdb.Machine,
	logger log.Logger) ([]byte, time.Time, error) {
	pathname := filepath.Join(g.directory,
		filepath.Clean(reflect.ValueOf(machine).FieldByIndex(g.index).String()))
	if data, err := ioutil.ReadFile(pathname); err != nil {
		if os.IsNotExist(err) {
			data, err := ioutil.ReadFile(filepath.Join(g.directory, "*"))
			if err == nil {
				return data, time.Time{}, nil
			}
		}
		return nil, time.Time{}, err
	} else {
		return data, time.Time{}, nil
	}
}
