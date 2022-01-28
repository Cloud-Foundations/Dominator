package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/cpulimiter"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/flags/loadflags"
	"github.com/Cloud-Foundations/Dominator/lib/flagutil"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsbench"
	"github.com/Cloud-Foundations/Dominator/lib/fsrateio"
	"github.com/Cloud-Foundations/Dominator/lib/goroutine"
	"github.com/Cloud-Foundations/Dominator/lib/html"
	"github.com/Cloud-Foundations/Dominator/lib/log/serverlogger"
	"github.com/Cloud-Foundations/Dominator/lib/memstats"
	"github.com/Cloud-Foundations/Dominator/lib/netspeed"
	"github.com/Cloud-Foundations/Dominator/lib/rateio"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/srpc/setupserver"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
	"github.com/Cloud-Foundations/Dominator/proto/sub"
	"github.com/Cloud-Foundations/Dominator/sub/httpd"
	"github.com/Cloud-Foundations/Dominator/sub/rpcd"
	"github.com/Cloud-Foundations/Dominator/sub/scanner"
	"github.com/Cloud-Foundations/tricorder/go/tricorder"
	"github.com/Cloud-Foundations/tricorder/go/tricorder/units"
)

var (
	configDirectory = flag.String("configDirectory", "/etc/subd/conf.d",
		"Directory of optional JSON configuration files")
	defaultCpuPercent = flag.Uint("defaultCpuPercent", 0,
		"CPU speed as percentage of capacity (default 50)")
	defaultNetworkSpeedPercent = flag.Uint("defaultNetworkSpeedPercent", 0,
		"Network speed as percentage of capacity (default 10)")
	defaultScanSpeedPercent = flag.Uint("defaultScanSpeedPercent", 0,
		"Scan speed as percentage of capacity (default 2)")
	maxThreads = flag.Uint("maxThreads", 1,
		"Maximum number of parallel OS threads to use")
	permitInsecureMode = flag.Bool("permitInsecureMode", false,
		"If true, run in insecure mode. This gives remote root access to all")
	pidfile = flag.String("pidfile", "/var/run/subd.pid",
		"Name of file to write my PID to")
	portNum = flag.Uint("portNum", constants.SubPortNumber,
		"Port number to allocate and listen on for HTTP/RPC")
	rootDeviceBytesPerSecond flagutil.Size
	rootDir                  = flag.String("rootDir", "/",
		"Name of root of directory tree to manage")
	scanExcludeList flagutil.StringList
	showStats       = flag.Bool("showStats", false,
		"If true, show statistics after each cycle")
	subdDir = flag.String("subdDir", ".subd",
		"Name of subd private directory, relative to rootDir. This must be on the same file-system as rootDir")
	testExternallyPatchable = flag.Bool("testExternallyPatchable", false,
		"If true, test if externally patchable and exit=0 if so or exit=1 if not")
)

func init() {
	// Ensure the main goroutine runs on the startup thread.
	runtime.LockOSThread()
	flag.Var(&rootDeviceBytesPerSecond, "rootDeviceBytesPerSecond",
		"Fallback root device speed (default 0)")
	flag.Var(&scanExcludeList, "scanExcludeList",
		`Comma separated list of patterns to exclude from scanning (default `+strings.Join(constants.ScanExcludeList, ",")+`")`)
}

func sanityCheck() bool {
	r_devnum, err := fsbench.GetDevnumForFile(*rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to get device number for: %s: %s\n",
			*rootDir, err)
		return false
	}
	subdDirPathname := path.Join(*rootDir, *subdDir)
	s_devnum, err := fsbench.GetDevnumForFile(subdDirPathname)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to get device number for: %s: %s\n",
			subdDirPathname, err)
		return false
	}
	if r_devnum != s_devnum {
		fmt.Fprintf(os.Stderr,
			"rootDir and subdDir must be on the same file-system\n")
		return false
	}
	return true
}

func createDirectory(dirname string) bool {
	if err := os.MkdirAll(dirname, 0750); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create directory: %s: %s\n",
			dirname, err)
		return false
	}
	return true
}

func mountTmpfs(dirname string) bool {
	var statfs syscall.Statfs_t
	if err := syscall.Statfs(dirname, &statfs); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create Statfs: %s: %s\n",
			dirname, err)
		return false
	}
	if statfs.Type != 0x01021994 {
		err := wsyscall.Mount("none", dirname, "tmpfs", 0,
			"size=65536,mode=0750")
		if err == nil {
			fmt.Printf("Mounted tmpfs on: %s\n", dirname)
		} else {
			fmt.Fprintf(os.Stderr, "Unable to mount tmpfs on: %s: %s\n",
				dirname, err)
			return false
		}
	}
	return true
}

func unshareAndBind(workingRootDir string) error {
	if err := wsyscall.UnshareMountNamespace(); err != nil {
		return fmt.Errorf("unable to unshare mount namesace: %s\n", err)
	}
	syscall.Unmount(workingRootDir, 0)
	err := wsyscall.Mount(*rootDir, workingRootDir, "", wsyscall.MS_BIND, "")
	if err != nil {
		return fmt.Errorf("unable to bind mount %s to %s: %s\n",
			*rootDir, workingRootDir, err)
	}
	return nil
}

func getCachedFsSpeed(workingRootDir string,
	cacheDirname string) (bytesPerSecond, blocksPerSecond uint64,
	computed, ok bool) {
	bytesPerSecond = 0
	blocksPerSecond = 0
	devnum, err := fsbench.GetDevnumForFile(workingRootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to get device number for: %s: %s\n",
			workingRootDir, err)
		return 0, 0, false, false
	}
	fsbenchDir := path.Join(cacheDirname, "fsbench")
	if !createDirectory(fsbenchDir) {
		return 0, 0, false, false
	}
	cacheFilename := path.Join(fsbenchDir, strconv.FormatUint(devnum, 16))
	file, err := os.Open(cacheFilename)
	if err == nil {
		n, err := fmt.Fscanf(file, "%d %d", &bytesPerSecond, &blocksPerSecond)
		file.Close()
		if n == 2 || err == nil {
			return bytesPerSecond, blocksPerSecond, false, true
		}
	}
	bytesPerSecond, blocksPerSecond, err = fsbench.GetReadSpeed(workingRootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to measure read speed: %s\n", err)
		return 0, 0, true, false
	}
	file, err = os.Create(cacheFilename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to open: %s for write: %s\n",
			cacheFilename, err)
		return 0, 0, true, false
	}
	fmt.Fprintf(file, "%d %d\n", bytesPerSecond, blocksPerSecond)
	file.Close()
	return bytesPerSecond, blocksPerSecond, true, true
}

func publishFsSpeed(bytesPerSecond, blocksPerSecond uint64) {
	tricorder.RegisterMetric("/root-read-speed", &bytesPerSecond,
		units.BytePerSecond, "read speed of root file-system media")
	tricorder.RegisterMetric("/root-block-read-speed", &blocksPerSecond,
		units.None, "read speed of root file-system media in blocks/second")
}

func getCachedNetworkSpeed(cacheFilename string) uint64 {
	if speed, ok := netspeed.GetSpeedToHost(""); ok {
		return speed
	}
	file, err := os.Open(cacheFilename)
	if err != nil {
		return 0
	}
	defer file.Close()
	var bytesPerSecond uint64
	n, err := fmt.Fscanf(file, "%d", &bytesPerSecond)
	if n == 1 || err == nil {
		return bytesPerSecond
	}
	return 0
}

type DumpableFileSystemHistory struct {
	fsh *scanner.FileSystemHistory
}

func (fsh *DumpableFileSystemHistory) WriteHtml(writer io.Writer) {
	fs := fsh.fsh.FileSystem()
	if fs == nil {
		return
	}
	fmt.Fprintln(writer, "<pre>")
	fs.List(writer)
	fmt.Fprintln(writer, "</pre>")
}

func gracefulCleanup() {
	os.Remove(*pidfile)
	os.Exit(1)
}

func writePidfile() {
	file, err := os.Create(*pidfile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	defer file.Close()
	fmt.Fprintln(file, os.Getpid())
}

func main() {
	// Ensure the startup thread is reserved for the main function.
	runtime.LockOSThread()
	if err := loadflags.LoadForDaemon("subd"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	flag.Parse()
	if *testExternallyPatchable {
		runTestAndExit(checkExternallyPatchable)
	}
	if err := wsyscall.SetMyPriority(1); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	tricorder.RegisterFlags()
	subdDirPathname := path.Join(*rootDir, *subdDir)
	workingRootDir := path.Join(subdDirPathname, "root")
	objectsDir := path.Join(workingRootDir, *subdDir, "objects")
	tmpDir := path.Join(subdDirPathname, "tmp")
	netbenchFilename := path.Join(subdDirPathname, "netbench")
	oldTriggersFilename := path.Join(subdDirPathname, "triggers.previous")
	if !createDirectory(workingRootDir) {
		os.Exit(1)
	}
	if !sanityCheck() {
		os.Exit(1)
	}
	if !createDirectory(tmpDir) {
		os.Exit(1)
	}
	if !mountTmpfs(tmpDir) {
		os.Exit(1)
	}
	// Create a goroutine for performing updates.
	workdirGoroutine := goroutine.New()
	var err error
	workdirGoroutine.Run(func() { err = unshareAndBind(workingRootDir) })
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	runtime.GOMAXPROCS(int(*maxThreads))
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
	bytesPerSecond, blocksPerSecond, firstScan, ok := getCachedFsSpeed(
		workingRootDir, tmpDir)
	if !ok {
		if rootDeviceBytesPerSecond < 1<<20 {
			os.Exit(1)
		}
		bytesPerSecond = uint64(rootDeviceBytesPerSecond)
		blocksPerSecond = bytesPerSecond >> 9
		logger.Printf("Falling back to -rootDeviceBytesPerSecond option: %s\n",
			format.FormatBytes(bytesPerSecond))
	}
	publishFsSpeed(bytesPerSecond, blocksPerSecond)
	configParams := sub.Configuration{}
	loadConfiguration(*configDirectory, &configParams, logger)
	// Command-line flags override file configuration.
	if *defaultCpuPercent > 0 {
		configParams.CpuPercent = *defaultCpuPercent
	}
	if *defaultNetworkSpeedPercent > 0 {
		configParams.NetworkSpeedPercent = *defaultNetworkSpeedPercent
	}
	if *defaultScanSpeedPercent > 0 {
		configParams.ScanSpeedPercent = *defaultScanSpeedPercent
	}
	var configuration scanner.Configuration
	configuration.CpuLimiter = cpulimiter.New(100)
	configuration.DefaultCpuPercent = configParams.CpuPercent
	// Apply built-in defaults if nothing specified.
	if configuration.DefaultCpuPercent < 1 {
		configuration.DefaultCpuPercent = constants.DefaultCpuPercent
		go adjustVcpuLimit(&configuration.DefaultCpuPercent, logger)
	}
	if configParams.NetworkSpeedPercent < 1 {
		configParams.NetworkSpeedPercent = constants.DefaultNetworkSpeedPercent
	}
	if configParams.ScanSpeedPercent < 1 {
		configParams.ScanSpeedPercent = constants.DefaultScanSpeedPercent
	}
	filterLines := configParams.ScanExclusionList
	if len(scanExcludeList) > 0 {
		filterLines = scanExcludeList
	}
	if len(filterLines) < 1 {
		filterLines = constants.ScanExcludeList
	}
	configuration.ScanFilter, err = filter.New(filterLines)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to set initial scan exclusions: %s\n",
			err)
		os.Exit(1)
	}
	configuration.FsScanContext = fsrateio.NewReaderContext(bytesPerSecond,
		blocksPerSecond, uint64(configParams.ScanSpeedPercent))
	defaultSpeed := configuration.FsScanContext.GetContext().SpeedPercent()
	if firstScan {
		configuration.FsScanContext.GetContext().SetSpeedPercent(100)
	}
	if *showStats {
		fmt.Println(configuration.FsScanContext)
	}
	var fsh scanner.FileSystemHistory
	mainFunc := func(fsChannel <-chan *scanner.FileSystem,
		disableScanner func(disableScanner bool)) {
		networkReaderContext := rateio.NewReaderContext(
			getCachedNetworkSpeed(netbenchFilename),
			uint64(configParams.NetworkSpeedPercent), &rateio.ReadMeasurer{})
		configuration.NetworkReaderContext = networkReaderContext
		invalidateNextScanObjectCache := false
		rescanFunc := func() {
			invalidateNextScanObjectCache = true
			if err := fsh.UpdateObjectCacheOnly(); err != nil {
				logger.Printf("Error updating object cache: %s\n", err)
			}
		}
		rpcdHtmlWriter := rpcd.Setup(
			rpcd.Config{
				NetworkBenchmarkFilename: netbenchFilename,
				ObjectsDirectoryName:     objectsDir,
				OldTriggersFilename:      oldTriggersFilename,
				RootDirectoryName:        workingRootDir,
				SubConfiguration:         configParams,
			},
			rpcd.Params{
				DisableScannerFunction:    disableScanner,
				FileSystemHistory:         &fsh,
				Logger:                    logger,
				NetworkReaderContext:      networkReaderContext,
				RescanObjectCacheFunction: rescanFunc,
				ScannerConfiguration:      &configuration,
				WorkdirGoroutine:          workdirGoroutine,
			})
		configMetricsDir, err := tricorder.RegisterDirectory("/config")
		if err != nil {
			fmt.Fprintf(os.Stderr,
				"Unable to create /config metrics directory: %s\n",
				err)
			os.Exit(1)
		}
		configuration.RegisterMetrics(configMetricsDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to create config metrics: %s\n", err)
			os.Exit(1)
		}
		httpd.AddHtmlWriter(rpcdHtmlWriter)
		httpd.AddHtmlWriter(&fsh)
		httpd.AddHtmlWriter(&configuration)
		httpd.AddHtmlWriter(logger)
		html.RegisterHtmlWriterForPattern("/dumpFileSystem",
			"Scanned File System",
			&DumpableFileSystemHistory{&fsh})
		if err = httpd.StartServer(*portNum, logger); err != nil {
			fmt.Fprintf(os.Stderr, "Unable to create http server: %s\n", err)
			os.Exit(1)
		}
		fsh.Update(nil)
		sighupChannel := make(chan os.Signal, 1)
		signal.Notify(sighupChannel, syscall.SIGHUP)
		sigtermChannel := make(chan os.Signal, 1)
		signal.Notify(sigtermChannel, syscall.SIGTERM, syscall.SIGINT)
		writePidfile()
		for iter := 0; true; {
			select {
			case <-sighupChannel:
				logger.Printf("Caught SIGHUP: re-execing with: %v\n", os.Args)
				logger.Flush()
				err = syscall.Exec(os.Args[0], os.Args, os.Environ())
				if err != nil {
					logger.Printf("Unable to Exec:%s: %s\n", os.Args[0], err)
				}
			case <-sigtermChannel:
				logger.Printf("Caught SIGTERM: performing graceful cleanup\n")
				logger.Flush()
				gracefulCleanup()
			case fs := <-fsChannel:
				if *showStats {
					fmt.Printf("Completed cycle: %d\n", iter)
				}
				if invalidateNextScanObjectCache {
					workdirGoroutine.Run(func() {
						if err := fs.ScanObjectCache(); err != nil {
							logger.Printf("Error scanning object cache: %s\n",
								err)
						}
					})
					invalidateNextScanObjectCache = false
				}
				fsh.Update(fs)
				iter++
				runtime.GC() // An opportune time to take out the garbage.
				if *showStats {
					fmt.Print(&fsh) // Use pointer to silence go vet.
					fmt.Print(fsh.FileSystem())
					memstats.WriteMemoryStats(os.Stdout)
					fmt.Println()
				}
				if firstScan {
					configuration.FsScanContext.GetContext().SetSpeedPercent(
						defaultSpeed)
					firstScan = false
					if *showStats {
						fmt.Println(configuration.FsScanContext)
					}
				}
			}
		}
	}
	// Create a goroutine prior to mutating the startup thread to ensure that
	// new goroutines are started from a "clean" thread.
	mainGoroutine := goroutine.New()
	// Setup environment for scanning.
	if err := unshareAndBind(workingRootDir); err != nil {
		logger.Fatalln(err)
	}
	if !createDirectory(objectsDir) { // Must be done after unshareAndBind().
		os.Exit(1)
	}
	scanner.StartScanning(workingRootDir, objectsDir, &configuration, logger,
		func(fsChannel <-chan *scanner.FileSystem,
			disableScanner func(disableScanner bool)) {
			mainGoroutine.Start(func() { mainFunc(fsChannel, disableScanner) })
		})
}
