/*
Package mdbd implements a simple MDB watcher.

Package mdbd may be used to read MDB data from a file or remote server and
watch for updates.
*/
package mdbd

import (
	"flag"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/debuglogger"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
)

var (
	mdbServerHostname = flag.String("mdbServerHostname", "",
		"Hostname of remote MDB server to get MDB updates from")
	mdbServerPortNum = flag.Uint("mdbServerPortNum",
		constants.SimpleMdbServerPortNumber, "Port number of MDB server")
)

type Config struct {
	MdbFileName       string
	MdbServerHostname string
	MdbServerPortNum  uint
}

type Params struct {
	Logger log.DebugLogger
}

// StartMdbDaemon is a convenience wrapper for StartMdbDaemon2. The
// -mdbServerHostname and -mdbServerPortNum command-line flags are used for the
// corresponding Params fields.
func StartMdbDaemon(mdbFileName string, logger log.Logger) <-chan *mdb.Mdb {
	return startMdbDaemon(
		Config{
			MdbFileName:       mdbFileName,
			MdbServerHostname: *mdbServerHostname,
			MdbServerPortNum:  *mdbServerPortNum,
		},
		Params{
			Logger: debuglogger.Upgrade(logger),
		})
}

// StartMdbDaemon2 starts an in-process "daemon" goroutine which watches for MDB
// updates. At startup it will read the file named by config.MdbFileName for
// MDB data. The default format is JSON, but if the filename extension is
// ".gob" then GOB format is read. If the file is present and contains MDB data,
// the MDB data are sent over the returned channel, otherwise no MDB data are
// sent initially.
//
// By default the file is monitored for updates and if the file is replaced by a
// different inode, MDB data are read from the new inode. If the MDB data are
// different than previously read, they are sent over the channel. This mode of
// operation is designed for consuming MDB data via the file-system from a local
// mdbd daemon.
//
// Alternatively, if a remote MDB server is specified with the
// config.MdbServerHostname and config.MdbServerPortNum fields then the remote
// MDB server is queried for MDB data. As MDB updates are received they are
// saved in the file and sent over the channel. In this mode of operation the
// file is read only once at startup, and is replaced when MDB updates are
// received. The file acts as a local cache of the MDB data received from the
// server, in case the MDB server is not available at a subsequent restart of
// the application.
//
// The params.Logger will be used to log problems.
func StartMdbDaemon2(config Config, params Params) <-chan *mdb.Mdb {
	return startMdbDaemon(config, params)
}
