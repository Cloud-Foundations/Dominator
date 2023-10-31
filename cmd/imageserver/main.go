package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Cloud-Foundations/Dominator/imageserver/httpd"
	imageserverRpcd "github.com/Cloud-Foundations/Dominator/imageserver/rpcd"
	"github.com/Cloud-Foundations/Dominator/imageserver/scanner"
	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/flags/loadflags"
	"github.com/Cloud-Foundations/Dominator/lib/log/serverlogger"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/srpc/setupserver"
	objectserverRpcd "github.com/Cloud-Foundations/Dominator/objectserver/rpcd"
	"github.com/Cloud-Foundations/tricorder/go/tricorder"
	"github.com/Cloud-Foundations/tricorder/go/tricorder/units"
)

var (
	allowPublicAddObjects = flag.Bool("allowPublicAddObjects", false,
		"If true, allow all users to call AddObjects method")
	allowPublicCheckObjects = flag.Bool("allowPublicCheckObjects", false,
		"If true, allow all users to call CheckObjects method")
	allowPublicGetObjects = flag.Bool("allowPublicGetObjects", false,
		"If true, allow all users to call GetObjects method")
	debug    = flag.Bool("debug", false, "If true, show debugging output")
	imageDir = flag.String("imageDir", "/var/lib/imageserver",
		"Name of image server data directory.")
	imageServerHostname = flag.String("imageServerHostname", "",
		"Hostname of image server to receive updates from")
	imageServerPortNum = flag.Uint("imageServerPortNum",
		constants.ImageServerPortNumber,
		"Port number of image server")
	lockCheckInterval = flag.Duration("lockCheckInterval", 2*time.Second,
		"Interval between checks for lock timeouts")
	lockLogTimeout = flag.Duration("lockLogTimeout", 5*time.Second,
		"Timeout before logging that a lock has been held too long")
	objectDir = flag.String("objectDir", "/var/lib/objectserver",
		"Name of image server data directory.")
	permitInsecureMode = flag.Bool("permitInsecureMode", false,
		"If true, run in insecure mode. This gives remote access to all")
	portNum = flag.Uint("portNum", constants.ImageServerPortNumber,
		"Port number to allocate and listen on for HTTP/RPC")
)

type imageObjectServersType struct {
	imageServerAddress string
	imdb               *scanner.ImageDataBase
	objSrv             *filesystem.ObjectServer
}

func main() {
	if os.Geteuid() == 0 {
		fmt.Fprintln(os.Stderr, "Do not run the Image Server as root")
		os.Exit(1)
	}
	if err := loadflags.LoadForDaemon("imageserver"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	flag.Parse()
	tricorder.RegisterFlags()
	logger := serverlogger.New("")
	srpc.SetDefaultLogger(logger)
	params := setupserver.Params{Logger: logger}
	if err := setupserver.SetupTlsWithParams(params); err != nil {
		if *permitInsecureMode {
			logger.Println(err)
		} else {
			logger.Fatalln(err)
		}
	}
	objSrv, err := filesystem.NewObjectServerWithConfigAndParams(
		filesystem.Config{
			BaseDirectory:     *objectDir,
			LockCheckInterval: *lockCheckInterval,
			LockLogTimeout:    *lockLogTimeout,
		},
		filesystem.Params{
			Logger: logger,
		})
	if err != nil {
		logger.Fatalf("Cannot create ObjectServer: %s\n", err)
	}
	var imageServerAddress string
	if *imageServerHostname != "" {
		imageServerAddress = fmt.Sprintf("%s:%d", *imageServerHostname,
			*imageServerPortNum)
	}
	imdb, err := scanner.Load(
		scanner.Config{
			BaseDirectory:     *imageDir,
			LockCheckInterval: *lockCheckInterval,
			LockLogTimeout:    *lockLogTimeout,
			ReplicationMaster: imageServerAddress,
		},
		scanner.Params{
			Logger:       logger,
			ObjectServer: objSrv,
		})
	if err != nil {
		logger.Fatalf("Cannot load image database: %s\n", err)
	}
	tricorder.RegisterMetric("/image-count",
		func() uint { return imdb.CountImages() },
		units.None, "number of images")
	imgSrvRpcHtmlWriter, err := imageserverRpcd.Setup(imdb, imageServerAddress,
		objSrv, logger)
	if err != nil {
		logger.Fatalln(err)
	}
	objSrvRpcHtmlWriter := objectserverRpcd.Setup(
		objectserverRpcd.Config{
			AllowPublicAddObjects:   *allowPublicAddObjects,
			AllowPublicCheckObjects: *allowPublicCheckObjects,
			AllowPublicGetObjects:   *allowPublicGetObjects,
			ReplicationMaster:       imageServerAddress,
		},
		objectserverRpcd.Params{
			Logger:       logger,
			ObjectServer: objSrv,
		})
	httpd.AddHtmlWriter(imdb)
	httpd.AddHtmlWriter(&imageObjectServersType{
		imageServerAddress: imageServerAddress,
		imdb:               imdb,
		objSrv:             objSrv,
	})
	httpd.AddHtmlWriter(imgSrvRpcHtmlWriter)
	httpd.AddHtmlWriter(objSrvRpcHtmlWriter)
	httpd.AddHtmlWriter(logger)
	if err = httpd.StartServer(*portNum, imdb, objSrv, false); err != nil {
		logger.Fatalf("Unable to create http server: %s\n", err)
	}
}
