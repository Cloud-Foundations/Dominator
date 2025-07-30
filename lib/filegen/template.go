package filegen

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver/memory"
)

type templateGenerator struct {
	objectServer    *memory.ObjectServer
	logger          log.Logger
	template        *template.Template
	notifierChannel chan<- string
}

func (m *Manager) registerTemplateFileForPath(pathname string,
	templateFile string, watchForUpdates bool) error {
	tgen := &templateGenerator{
		objectServer: m.objectServer,
		logger:       m.logger}
	tgen.notifierChannel = m.registerHashGeneratorForPath(pathname, tgen)
	if watchForUpdates {
		readCloserChannel := fsutil.WatchFile(templateFile, m.logger)
		go tgen.handleReadClosers(readCloserChannel)
	} else {
		file, err := os.Open(templateFile)
		if err != nil {
			return err
		}
		if err := tgen.handleReadCloser(file); err != nil {
			return err
		}
	}
	return nil
}

func (tgen *templateGenerator) generate(machine mdb.Machine,
	logger log.Logger) (
	hash.Hash, uint64, time.Time, error) {
	if tgen.template == nil {
		return hash.Hash{}, 0, time.Time{}, errors.New("no template data yet")
	}
	buffer := new(bytes.Buffer)
	if err := tgen.template.Execute(buffer, machine); err != nil {
		return hash.Hash{}, 0, time.Time{}, err
	}
	length := uint64(buffer.Len())
	hashVal, _, err := tgen.objectServer.AddObject(buffer, length, nil)
	return hashVal, length, time.Time{}, err
}

func (tgen *templateGenerator) handleReadClosers(
	readCloserChannel <-chan io.ReadCloser) {
	for readCloser := range readCloserChannel {
		if err := tgen.handleReadCloser(readCloser); err != nil {
			tgen.logger.Println(err)
		}
	}
}

func (tgen *templateGenerator) handleReadCloser(
	readCloser io.ReadCloser) error {
	data, err := ioutil.ReadAll(readCloser)
	readCloser.Close()
	if err != nil {
		return err
	}
	funcMap := template.FuncMap{
		"GetSplitPart": func(s string, sep string, index int) string {
			parts := strings.Split(s, sep)
			if index >= 0 && index < len(parts) {
				return parts[index]
			}
			return ""
		},
		"ToLower": func(s string) string {
			if len(s) > 0 {
				return strings.ToLower(s)
			}
			return ""
		},
		"ToUpper": func(s string) string {
			if len(s) > 0 {
				return strings.ToUpper(s)
			}
			return ""
		},
	}
	tmpl, err := template.New("generatorTemplate").Funcs(funcMap).Parse(string(data))
	if err != nil {
		return err
	}
	tgen.template = tmpl
	tgen.notifierChannel <- ""
	return nil
}
