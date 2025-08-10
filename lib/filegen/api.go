/*
Package filegen manages the generation of computed files.

Package filegen may be used to implement a computed file server. It
registers a FileGenerator server with the lib/srpc package. The application
may register multiple file generators.

A generator for the /etc/mdb.json pathname is automatically registered.
*/
package filegen

import (
	"io"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver/memory"
	proto "github.com/Cloud-Foundations/Dominator/proto/filegenerator"
	"github.com/Cloud-Foundations/tricorder/go/tricorder"
)

// FileGenerator is the interface that wraps the Generate method.
//
// Generate computes file data from the provided machine information.
// The logger may be used to log problems.
// It returns the data, a time.Time indicating when the data are valid until
// (a zero time indicates the data are valid forever) and an error.
type FileGenerator interface {
	Generate(machine mdb.Machine, logger log.Logger) (
		data []byte, validUntil time.Time, err error)
}

type expiringHash struct {
	hash       hash.Hash
	length     uint64
	validUntil time.Time
}

type pathManager struct {
	distributionFailed     *tricorder.CumulativeDistribution
	distributionSuccessful *tricorder.CumulativeDistribution
	generator              hashGenerator
	rwMutex                sync.RWMutex
	// Protected by lock.
	machineHashes map[string]expiringHash // Key: hostname.
}

type Manager struct {
	rwMutex sync.RWMutex
	// Protected by lock.
	pathManagers map[string]*pathManager // Key: pathname.
	machineData  map[string]mdb.Machine  // Key: hostname.
	clients      map[<-chan *proto.ServerMessage]chan<- *proto.ServerMessage
	// Not protected by lock.
	bucketer     *tricorder.Bucketer
	objectServer *memory.ObjectServer
	logger       log.DebugLogger
}

// New creates a new *Manager. Only one should be created per application.
// The logger will be used to log problems.
func New(logger log.Logger) *Manager {
	return newManager(logger)
}

// GetRegisteredPaths returns a slice of filenames which have generators.
func (m *Manager) GetRegisteredPaths() []string {
	return m.getRegisteredPaths()
}

// RegisterFileForPath registers a source file for a specific pathname. The
// source file is used as the data source. If the source file changes, the data
// are re-read.
func (m *Manager) RegisterFileForPath(pathname string, sourceFile string) {
	m.registerFileForPath(pathname, sourceFile)
}

// RegisterGeneratorForPath registers a FileGenerator for a specific pathname.
// It returns a channel to which notification messages may be sent indicating
// that the data should be regenerated, even if the machine data have not
// changed. If the empty string is sent to the channel, it indicates that data
// should be regenerated for all machines, otherwise it indicates that data
// should be regenerated for a specific machine.
// An internal goroutine reads from the channel, which terminates if the channel
// is closed. The channel should be closed if the data should only be
// regenerated if the machine data change.
func (m *Manager) RegisterGeneratorForPath(pathname string,
	gen FileGenerator) chan<- string {
	return m.registerDataGeneratorForPath(pathname, gen)
}

// RegisterMdbFieldDirectoryForPath registers a generator for pathname which
// yields data from files under the specified directory where the filenames
// are taken from the specified field of the MDB data for a machine. The data
// are periodically reloaded every specified interval (if greater than zero).
func (m *Manager) RegisterMdbFieldDirectoryForPath(pathname string,
	field, directory string, interval time.Duration) error {
	return m.registerMdbFieldDirectoryForPath(pathname, field, directory,
		interval)
}

// RegisterMdbGeneratorForPath registers a generator for pathname which yields
// the MDB data in JSON format for a machine.
func (m *Manager) RegisterMdbGeneratorForPath(pathname string) {
	m.registerMdbGeneratorForPath(pathname)
}

// RegisterProgrammeForPath registers a programme which will be called to
// generate data for the specified pathname. The pathname will be provided as
// the first argument to the programme.
// The MDB data for the machine will be written to the standard input in JSON
// format.
// The data and number of seconds it is valid (0 means indefinitely valid) must
// be written to the standard output in JSON format, stored in the Data and
// SecondsValid fields.
func (m *Manager) RegisterProgrammeForPath(pathname, programmePath string) {
	m.registerProgrammeForPath(pathname, programmePath)
}

// RegisterTemplateFileForPath registers a template file for a specific
// pathname.
// The template file is used to generate the data, modified by the machine data.
// If the template file changes and watchForUpdates is true, the template file
// is re-read and the data are regenerated.
// The template file syntax is defined by the text/template standard package.
func (m *Manager) RegisterTemplateFileForPath(pathname string,
	templateFile string, watchForUpdates bool) error {
	return m.registerTemplateFileForPath(pathname, TemplateFileConfig{
		TemplateFile:    templateFile,
		WatchForUpdates: watchForUpdates,
	})
}

// RegisterTemplateFileForPathWithConfig registers a template file for a
// specific pathname.
// The template file is used to generate the data, modified by the machine data.
// If the template file changes and WatchForUpdates is true, the template file
// is re-read and the data are regenerated.
// The template file syntax is defined by the text/template standard package.
func (m *Manager) RegisterTemplateFileForPathWithConfig(pathname string,
	config TemplateFileConfig) error {
	return m.registerTemplateFileForPath(pathname, config)
}

// RegisterUrlForPath registers a URL where a HTTP POST request may be sent to
// generate data for the specified pathname. The pathname will be provided in
// a pathname= query parameter.
// The MDB data for the machine will be written to the message body in JSON
// format.
// The data and number of seconds it is valid (0 means indefinitely valid) must
// be returned in the response body in JSON format, stored in the Data and
// SecondsValid fields.
func (m *Manager) RegisterUrlForPath(pathname, URL string) {
	m.registerUrlForPath(pathname, URL)
}

// WriteHtml will write status information about the Manager to w, with
// appropriate HTML markups.
func (m *Manager) WriteHtml(writer io.Writer) {
	m.writeHtml(writer)
}

type TemplateFileConfig struct {
	TemplateFile    string
	VariablesFile   string
	WatchForUpdates bool
}
