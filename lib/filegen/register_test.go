package filegen

import (
	"fmt"
	"testing"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/testlogger"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
)

type testGenerator struct{}

var testData = []byte("data")

func (g *testGenerator) Generate(machine mdb.Machine, logger log.Logger) (
	data []byte, validUntil time.Time, err error) {
	return testData, time.Now().Add(time.Minute), nil
}

func TestManyRegisters(t *testing.T) {
	m := New(testlogger.New(t))
	dataGenerator := &testGenerator{}
	var pathnames []string
	for count := 0; count < 100; count++ {
		pathnames = append(pathnames, fmt.Sprintf("dir/file%d", count))
	}
	for _, pathname := range pathnames {
		m.RegisterGeneratorForPath(pathname, dataGenerator)
	}
}
