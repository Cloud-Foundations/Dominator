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
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver/memory"
)

var funcMap = template.FuncMap{
	"Contains":     strings.Contains,
	"GetSplitPart": getSplitPart,
	"ToLower":      strings.ToLower,
	"ToUpper":      strings.ToUpper,
}

type templateGenerator struct {
	objectServer    *memory.ObjectServer
	logger          log.Logger
	template        *template.Template
	variables       map[string]string
	notifierChannel chan<- string
}

func (m *Manager) registerTemplateFileForPath(pathname string,
	config TemplateFileConfig) error {
	tgen := &templateGenerator{
		objectServer: m.objectServer,
		logger:       m.logger}
	tgen.notifierChannel = m.registerHashGeneratorForPath(pathname, tgen)
	if config.VariablesFile != "" {
		// TODO(rgooch): consider making this watch for updates.
		variables := make(map[string]string)
		err := json.ReadFromFile(config.VariablesFile, &variables)
		if err != nil {
			return err
		}
		tgen.variables = variables
	}
	if config.WatchForUpdates {
		readCloserChannel := fsutil.WatchFile(config.TemplateFile, m.logger)
		go tgen.handleReadClosers(readCloserChannel)
	} else {
		file, err := os.Open(config.TemplateFile)
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

func getSplitPart(s string, sep string, index int) string {
	parts := strings.Split(s, sep)
	if index >= 0 && index < len(parts) {
		return parts[index]
	}
	return ""
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
	tmpl := template.New("generatorTemplate").Funcs(funcMap)
	if tgen.variables != nil {
		tmpl.Funcs(template.FuncMap{"LookupGeneratorVariable": tgen.lookup})
	}
	tmpl, err = tmpl.Parse(string(data))
	if err != nil {
		return err
	}
	tgen.template = tmpl
	tgen.notifierChannel <- ""
	return nil
}

func (tgen *templateGenerator) lookup(s string) string {
	return tgen.variables[s]
}
