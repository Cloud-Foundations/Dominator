package rpcd

import (
	"errors"
	"flag"
	"io"
	"sync"

	"github.com/Cloud-Foundations/Dominator/imageserver/scanner"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

var (
	archiveExpiringImages = flag.Bool("archiveExpiringImages", false,
		"If true, replicate expiring images when in archive mode")
	archiveMode = flag.Bool("archiveMode", false,
		"If true, disable delete operations and require update server")
	replicationExcludeFilter = flag.String("replicationExcludeFilter", "",
		"Filename containing filter to exclude images from replication (default do not exclude any)")
	replicationIncludeFilter = flag.String("replicationIncludeFilter", "",
		"Filename containing filter to include images for replication (default include all)")
)

type srpcType struct {
	imageDataBase             *scanner.ImageDataBase
	excludeFilter             *filter.Filter
	finishedReplication       <-chan struct{} // Closed when finished.
	includeFilter             *filter.Filter
	replicationMaster         string
	imageserverResource       *srpc.ClientResource
	objSrv                    objectserver.FullObjectServer
	archiveMode               bool
	logger                    log.DebugLogger
	numReplicationClientsLock sync.RWMutex // Protect numReplicationClients.
	numReplicationClients     uint
	imagesBeingInjectedLock   sync.Mutex // Protect imagesBeingInjected.
	imagesBeingInjected       map[string]struct{}
}

type htmlWriter srpcType

func (hw *htmlWriter) WriteHtml(writer io.Writer) {
	hw.writeHtml(writer)
}

var replicationMessage = "cannot make changes while under replication control" +
	", go to master: "

func Setup(imdb *scanner.ImageDataBase, replicationMaster string,
	objSrv objectserver.FullObjectServer,
	logger log.DebugLogger) (*htmlWriter, error) {
	if *archiveMode && replicationMaster == "" {
		return nil, errors.New("replication master required in archive mode")
	}
	finishedReplication := make(chan struct{})
	srpcObj := &srpcType{
		imageDataBase:       imdb,
		finishedReplication: finishedReplication,
		replicationMaster:   replicationMaster,
		imageserverResource: srpc.NewClientResource("tcp", replicationMaster),
		objSrv:              objSrv,
		logger:              logger,
		archiveMode:         *archiveMode,
		imagesBeingInjected: make(map[string]struct{}),
	}
	var err error
	if *replicationExcludeFilter != "" {
		srpcObj.excludeFilter, err = filter.Load(*replicationExcludeFilter)
		if err != nil {
			return nil, err
		}
	}
	if *replicationIncludeFilter != "" {
		srpcObj.includeFilter, err = filter.Load(*replicationIncludeFilter)
		if err != nil {
			return nil, err
		}
	}
	srpc.RegisterNameWithOptions("ImageServer", srpcObj, srpc.ReceiverOptions{
		PublicMethods: []string{
			"ChangeImageExpiration",
			"CheckDirectory",
			"CheckImage",
			"ChownDirectory",
			"DeleteImage",
			"FindLatestImage",
			"GetFilteredImageUpdates",
			"GetImage",
			"GetImageArchive",
			"GetImageComputedFiles",
			"GetImageExpiration",
			"GetImageUpdates",
			"GetReplicationMaster",
			"ListDirectories",
			"ListImages",
			"ListSelectedImages",
		}})
	if replicationMaster != "" {
		go srpcObj.replicator(finishedReplication)
	} else {
		close(finishedReplication)
	}
	return (*htmlWriter)(srpcObj), nil
}
