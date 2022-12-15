package filegen

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
	tagKey    string
}

func makeGenerator(field string) (*mdbFieldDirectoryType, error) {
	if fParts := strings.Split(field, "."); len(fParts) == 2 {
		if fParts[0] == "Tags" {
			return &mdbFieldDirectoryType{tagKey: fParts[1]}, nil
		}
	}
	structField, found := machineType.FieldByName(field)
	if !found {
		return nil, fmt.Errorf(
			"field: \"%s\" not found in mdb.Machine type", field)
	}
	if structField.Type.Kind() != reflect.String {
		return nil, fmt.Errorf(
			"field: \"%s\" is not string type", field)
	}
	return &mdbFieldDirectoryType{index: structField.Index}, nil
}

func sendNotifications(notifierChannel chan<- string,
	interval time.Duration) {
	for ; ; time.Sleep(interval) {
		notifierChannel <- ""
	}
}

func (m *Manager) registerMdbFieldDirectoryForPath(pathname string,
	field, directory string, interval time.Duration) error {
	generator, err := makeGenerator(field)
	if err != nil {
		return err
	}
	generator.directory = directory
	notifierChannel := m.RegisterGeneratorForPath(pathname,
		generator)
	if interval <= 0 {
		close(notifierChannel)
		return nil
	}
	go sendNotifications(notifierChannel, interval)
	return nil
}

func (g *mdbFieldDirectoryType) Generate(machine mdb.Machine,
	logger log.Logger) ([]byte, time.Time, error) {
	var fieldValue string
	if g.tagKey != "" {
		fieldValue = machine.Tags[g.tagKey]
	} else {
		fieldValue = reflect.ValueOf(machine).FieldByIndex(
			g.index).String()
	}
	if fieldValue == "" {
		fieldValue = "*"
	}
	pathname := filepath.Join(g.directory,
		filepath.Clean(fieldValue))
	if data, err := ioutil.ReadFile(pathname); err != nil {
		if os.IsNotExist(err) && fieldValue != "*" {
			data, err := ioutil.ReadFile(filepath.Join(g.directory,
				"*"))
			if err == nil {
				return data, time.Time{}, nil
			}
		}
		return nil, time.Time{}, err
	} else {
		return data, time.Time{}, nil
	}
}
