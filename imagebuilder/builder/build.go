package builder

import (
	"bytes"
	stderrors "errors"
	"fmt"
	"io"
	"time"

	buildclient "github.com/Cloud-Foundations/Dominator/imagebuilder/client"
	"github.com/Cloud-Foundations/Dominator/imagebuilder/logarchiver"
	imgclient "github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/backoffdelay"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/retry"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/srpc/retryclient"
	"github.com/Cloud-Foundations/Dominator/lib/url/urlutil"
	proto "github.com/Cloud-Foundations/Dominator/proto/imaginator"
)

type dualBuildLogger struct {
	buffer *bytes.Buffer
	writer io.Writer
}

type buildErrorType struct {
	error                  string
	needSourceImage        bool
	sourceImage            string
	sourceImageGitCommitId string
}

func copyClientLogs(clientAddress string, keepSlave bool, buildError error,
	buildLog io.Writer) {
	fmt.Fprintln(buildLog,
		"*********************************************************************")
	fmt.Fprintf(buildLog, "Logs for slave: %s, keepSlave: %v, err: %v\n",
		clientAddress, keepSlave, buildError)
	readCloser, err := urlutil.Open(
		fmt.Sprintf("http://%s/logs/dump?name=latest", clientAddress))
	if err != nil {
		fmt.Fprintf(buildLog, "Error opening logs: %s\n", err)
		return
	}
	defer readCloser.Close()
	io.Copy(buildLog, readCloser)
	fmt.Fprintln(buildLog, "End logs for slave")
	fmt.Fprintln(buildLog,
		"*********************************************************************")
}

func dialServer(address string, retryTimeout time.Duration) (
	srpc.ClientI, error) {
	return retryclient.DialHTTP(retryclient.Params{
		Address:         address,
		KeepAlive:       true,
		KeepAlivePeriod: time.Minute,
		Network:         "tcp",
		Params: retry.Params{
			RetryTimeout: retryTimeout,
			Sleeper:      backoffdelay.NewExponential(0, 0, 1),
		},
	})
}

// checkToAutoExtend will return true if auto building images should have their
// lifetimes extended rather than built. Some conditions where it returns true:
// - Auto building has been disabled.
// - A slave driver is configured but failes to provide a slave in time
func (b *Builder) checkToAutoExtend() bool {
	b.disableLock.RLock()
	disableUntil := b.disableAutoBuildsUntil
	b.disableLock.RUnlock()
	if duration := time.Until(disableUntil); duration > 0 {
		b.logger.Printf("Auto rebuilds disabled until %s (for %s), will extend image lifetimes\n",
			disableUntil.Format(format.TimeFormatSeconds),
			format.Duration(duration))
		return true
	}
	if b.slaveDriver == nil {
		return false
	}
	slave, err := b.slaveDriver.GetSlaveWithTimeout(b.createSlaveTimeout)
	if err == nil {
		slave.Release()
		return false
	}
	b.logger.Printf("Error getting slave: %s, will extend image lifetimes\n",
		err)
	return true
}

func (b *Builder) build(client srpc.ClientI, request proto.BuildImageRequest,
	authInfo *srpc.AuthInformation,
	logWriter io.Writer) (*image.Image, string, error) {
	startTime := time.Now()
	builder := b.getImageBuilderWithReload(request.StreamName)
	if builder == nil {
		return nil, "", errors.New("unknown stream: " + request.StreamName)
	}
	if err := b.checkPermission(builder, request, authInfo); err != nil {
		return nil, "", err
	}
	buildLogBuffer := &bytes.Buffer{}
	buildInfo := &currentBuildInfo{
		buffer:    buildLogBuffer,
		startedAt: time.Now(),
	}
	b.buildResultsLock.Lock()
	b.currentBuildInfos[request.StreamName] = buildInfo
	b.buildResultsLock.Unlock()
	var buildLog buildLogger
	if logWriter == nil {
		buildLog = buildLogBuffer
	} else {
		buildLog = &dualBuildLogger{
			buffer: buildLogBuffer,
			writer: io.MultiWriter(buildLogBuffer, logWriter),
		}
	}
	img, name, err := b.buildWithLogger(builder, client, request, authInfo,
		startTime, &buildInfo.slaveAddress, buildLog)
	finishTime := time.Now()
	b.buildResultsLock.Lock()
	defer b.buildResultsLock.Unlock()
	delete(b.currentBuildInfos, request.StreamName)
	b.lastBuildResults[request.StreamName] = buildResultType{
		name, startTime, finishTime, buildLog.Bytes(), err}
	buildLogInfo := logarchiver.BuildInfo{
		Duration: finishTime.Sub(startTime),
		Error:    errors.ErrorToString(err),
	}
	buildLogImageName := name
	if buildLogImageName == "" {
		buildLogImageName = makeImageName(request.StreamName)
	}
	if authInfo != nil {
		buildLogInfo.RequestorUsername = authInfo.Username
	}
	archiveError := b.buildLogArchiver.AddBuildLog(buildLogImageName,
		buildLogInfo, buildLog.Bytes())
	if archiveError != nil {
		b.logger.Printf("Error archiving build log: %s\n", archiveError)
	}
	if err == nil {
		b.logger.Printf("Built image for stream: %s in %s\n",
			request.StreamName, format.Duration(finishTime.Sub(startTime)))
	}
	return img, name, err
}

func (b *Builder) buildImage(request proto.BuildImageRequest,
	authInfo *srpc.AuthInformation,
	logWriter io.Writer) (*image.Image, string, error) {
	b.disableLock.RLock()
	disableUntil := b.disableBuildRequestsUntil
	b.disableLock.RUnlock()
	if duration := time.Until(disableUntil); duration > 0 {
		return nil, "", fmt.Errorf("builds disabled until %s (for %s)",
			disableUntil.Format(format.TimeFormatSeconds),
			format.Duration(duration))
	}
	if request.ExpiresIn < b.minimumExpiration {
		return nil, "", fmt.Errorf("minimum expiration duration is %s",
			format.Duration(b.minimumExpiration))
	}
	if err := b.WaitForStreamsLoaded(time.Minute); err != nil {
		return nil, "", err
	}
	client, err := dialServer(b.imageServerAddress, time.Minute)
	if err != nil {
		return nil, "", err
	}
	defer client.Close()
	img, name, err := b.build(client, request, authInfo, logWriter)
	if request.ReturnImage {
		return img, "", err
	}
	return nil, name, err
}

func (b *Builder) buildLocal(builder imageBuilder, client srpc.ClientI,
	request proto.BuildImageRequest, authInfo *srpc.AuthInformation,
	buildLog buildLogger) (*image.Image, error) {
	// Check the namespace to make sure it hasn't changed. This is to catch
	// golang bugs.
	currentNamespace, err := getNamespace()
	if err != nil {
		fmt.Fprintln(buildLog, err)
		return nil, err
	}
	if currentNamespace != b.initialNamespace {
		err := fmt.Errorf("namespace changed from: %s to: %s",
			b.initialNamespace, currentNamespace)
		fmt.Fprintln(buildLog, err)
		return nil, err
	}
	if authInfo == nil {
		b.logger.Printf("Auto building image for stream: %s\n",
			request.StreamName)
	} else {
		b.logger.Printf("%s requested building image for stream: %s\n",
			authInfo.Username, request.StreamName)
	}
	img, err := builder.build(b, client, request, buildLog)
	if err != nil {
		fmt.Fprintf(buildLog, "Error building image: %s\n", err)
		return nil, err
	}
	return img, nil
}

func (b *Builder) buildOnSlave(client srpc.ClientI,
	request proto.BuildImageRequest, authInfo *srpc.AuthInformation,
	slaveAddress *string, buildLog buildLogger) (*image.Image, error) {
	request.DisableRecursiveBuild = true
	request.ReturnImage = true
	request.StreamBuildLog = true
	if len(request.Variables) < 1 {
		request.Variables = b.variables
	} else if len(b.variables) > 0 {
		variables := make(map[string]string,
			len(b.variables)+len(request.Variables))
		for key, value := range b.variables {
			variables[key] = value
		}
		for key, value := range request.Variables {
			variables[key] = value
		}
		request.Variables = variables
	}
	slave, err := b.slaveDriver.GetSlaveWithTimeout(b.createSlaveTimeout)
	if err != nil {
		return nil, fmt.Errorf("error getting slave: %s", err)
	}
	*slaveAddress = slave.GetClientAddress()
	keepSlave := false
	defer func() {
		if keepSlave {
			slave.Release()
		} else {
			slave.Destroy()
		}
	}()
	if authInfo == nil {
		b.logger.Printf("Auto building image on %s for stream: %s\n",
			slave, request.StreamName)
		fmt.Fprintf(buildLog, "Auto building image on %s for stream: %s\n",
			slave, request.StreamName)
	} else {
		b.logger.Printf("%s requested building image on %s for stream: %s\n",
			authInfo.Username, slave, request.StreamName)
		fmt.Fprintf(buildLog,
			"%s requested building image on %s for stream: %s\n",
			authInfo.Username, slave, request.StreamName)
	}
	var reply proto.BuildImageResponse
	err = buildclient.BuildImage(slave.GetClient(), request, &reply, buildLog)
	copyClientLogs(slave.GetClientAddress(), keepSlave, err, buildLog)
	if err != nil {
		if reply.NeedSourceImage {
			return nil, &buildErrorType{
				error:                  err.Error(),
				needSourceImage:        true,
				sourceImage:            reply.SourceImage,
				sourceImageGitCommitId: reply.SourceImageGitCommitId,
			}
		}
		return nil, err
	}
	return reply.Image, nil
}

func (b *Builder) buildSomewhere(builder imageBuilder, client srpc.ClientI,
	request proto.BuildImageRequest, authInfo *srpc.AuthInformation,
	slaveAddress *string, buildLog buildLogger) (*image.Image, error) {
	if b.slaveDriver == nil {
		return b.buildLocal(builder, client, request, authInfo, buildLog)
	} else {
		return b.buildOnSlave(client, request, authInfo, slaveAddress, buildLog)
	}
}

func (b *Builder) buildWithLogger(builder imageBuilder, client srpc.ClientI,
	request proto.BuildImageRequest, authInfo *srpc.AuthInformation,
	startTime time.Time, slaveAddress *string,
	buildLog buildLogger) (*image.Image, string, error) {
	img, err := b.buildSomewhere(builder, client, request, authInfo,
		slaveAddress, buildLog)
	if err != nil {
		var buildError *buildErrorType
		if stderrors.As(err, &buildError) && buildError.needSourceImage {
			if request.DisableRecursiveBuild {
				return nil, "", err
			}
			// Try to build source image.
			expiresIn := time.Hour
			if request.ExpiresIn > 0 {
				expiresIn = request.ExpiresIn
			}
			sourceReq := proto.BuildImageRequest{
				ExpiresIn:    expiresIn,
				GitBranch:    buildError.sourceImageGitCommitId,
				MaxSourceAge: request.MaxSourceAge,
				StreamName:   buildError.sourceImage,
				Variables:    request.Variables,
			}
			if _, _, e := b.build(client, sourceReq, nil, buildLog); e != nil {
				return nil, "", e
			}
			img, err = b.buildSomewhere(builder, client, request, authInfo,
				slaveAddress, buildLog)
		}
	}
	if err != nil {
		return nil, "", err
	}
	if request.ReturnImage {
		return img, "", nil
	}
	if authInfo != nil {
		img.CreatedFor = authInfo.Username
	}
	uploadStartTime := time.Now()
	if name, err := addImage(client, request, img); err != nil {
		fmt.Fprintln(buildLog, err)
		return nil, "", err
	} else {
		finishTime := time.Now()
		fmt.Fprintf(buildLog,
			"Uploaded %s in %s, total build duration: %s\n",
			name, format.Duration(finishTime.Sub(uploadStartTime)),
			format.Duration(finishTime.Sub(startTime)))
		return img, name, nil
	}
}

func (b *Builder) checkPermission(builder imageBuilder,
	request proto.BuildImageRequest, authInfo *srpc.AuthInformation) error {
	if authInfo == nil || authInfo.HaveMethodAccess {
		if request.ExpiresIn > b.maximumExpirationPrivileged {
			return fmt.Errorf("maximum expiration time is %s for you",
				format.Duration(b.maximumExpirationPrivileged))
		}
		return nil
	}
	if request.ExpiresIn > b.maximumExpiration {
		return fmt.Errorf("maximum expiration time is %s",
			format.Duration(b.maximumExpiration))
	}
	if builder, ok := builder.(*imageStreamType); ok {
		if _, ok := builder.builderUsers[authInfo.Username]; ok {
			return nil
		}
		for _, group := range builder.BuilderGroups {
			if _, ok := authInfo.GroupList[group]; ok {
				return nil
			}
		}
	}
	return errors.New("no permission to build: " + request.StreamName)
}

func (b *Builder) disableAutoBuilds(disableFor time.Duration) (
	time.Time, error) {
	if b.imageRebuildInterval > 0 {
		if disableFor > b.imageRebuildInterval<<2 {
			return time.Time{},
				fmt.Errorf("cannot disable auto building for more than: %s",
					format.Duration(b.imageRebuildInterval<<2))
		}
	}
	b.disableLock.Lock()
	defer b.disableLock.Unlock()
	b.disableAutoBuildsUntil = time.Now().Add(disableFor)
	return b.disableAutoBuildsUntil, nil
}

func (b *Builder) disableBuildRequests(disableFor time.Duration) (
	time.Time, error) {
	if disableFor > 24*time.Hour {
		return time.Time{},
			fmt.Errorf("cannot disable build requests for more than: 24h")
	}
	b.disableLock.Lock()
	defer b.disableLock.Unlock()
	b.disableBuildRequestsUntil = time.Now().Add(disableFor)
	return b.disableBuildRequestsUntil, nil
}

// extendAutoRebuildImages will return true if image lifetimes were extended and
// thus auto rebuilding should be skipped.
func (b *Builder) extendAutoRebuildImages(client srpc.ClientI,
	expiresIn time.Duration) bool {
	if !b.checkToAutoExtend() {
		return false
	}
	for _, streamName := range b.listStreamsToAutoRebuild() {
		imageName, err := imgclient.FindLatestImage(client, streamName, false)
		if err != nil {
			b.logger.Printf("Error finding latest image for stream: %s: %s\n",
				streamName, err)
			continue
		}
		if imageName == "" {
			continue
		}
		err = imgclient.ChangeImageExpiration(client, imageName,
			time.Now().Add(expiresIn))
		if err != nil {
			b.logger.Printf("Error extending expiration for image: %s: %s\n",
				imageName, err)
		}
	}
	return true
}

func (b *Builder) getCurrentBuildLog(streamName string) ([]byte, error) {
	b.buildResultsLock.RLock()
	defer b.buildResultsLock.RUnlock()
	if result, ok := b.currentBuildInfos[streamName]; !ok {
		return nil, errors.New("unknown image: " + streamName)
	} else {
		log := make([]byte, result.buffer.Len())
		copy(log, result.buffer.Bytes())
		return log, nil
	}
}

func (b *Builder) getImageBuilder(streamName string) imageBuilder {
	if stream := b.getBootstrapStream(streamName); stream != nil {
		return stream
	}
	if stream := b.getNormalStream(streamName); stream != nil {
		return stream
	}
	// Ensure a nil interface is returned, not a stream with value == nil.
	return nil
}

func (b *Builder) getImageBuilderWithReload(streamName string) imageBuilder {
	if stream := b.getImageBuilder(streamName); stream != nil {
		return stream
	}
	if err := b.reloadNormalStreamsConfiguration(); err != nil {
		b.logger.Printf("Error reloading configuration: %s\n", err)
		return nil
	}
	return b.getImageBuilder(streamName)
}

func (b *Builder) getLatestBuildLog(streamName string) ([]byte, error) {
	b.buildResultsLock.RLock()
	defer b.buildResultsLock.RUnlock()
	if result, ok := b.lastBuildResults[streamName]; !ok {
		return nil, errors.New("unknown image: " + streamName)
	} else {
		log := make([]byte, len(result.buildLog))
		copy(log, result.buildLog)
		return log, nil
	}
}

func (b *Builder) rebuildImage(client srpc.ClientI, streamName string,
	expiresIn time.Duration) {
	_, _, err := b.build(client, proto.BuildImageRequest{
		StreamName: streamName,
		ExpiresIn:  expiresIn,
	},
		nil, nil)
	if err == nil {
		return
	}
	imageName, e := imgclient.FindLatestImage(client, streamName, false)
	if e != nil {
		b.logger.Printf("Error finding latest image: %s: %s\n", streamName, e)
	} else if imageName != "" {
		e := imgclient.ChangeImageExpiration(client, imageName,
			time.Now().Add(expiresIn))
		if e == nil {
			b.logger.Printf("Error building image: %s: %s, extended: %s\n",
				streamName, err, imageName)
			return
		}
		b.logger.Printf(
			"Error building image: %s: %s, failed to extend: %s: %s\n",
			streamName, err, imageName, e)
		return
	}
	b.logger.Printf("Error building image: %s: %s\n", streamName, err)
}

func (b *Builder) rebuildImages(minInterval time.Duration) {
	if minInterval < 1 {
		return
	}
	client, _ := dialServer(b.imageServerAddress, 0)
	var sleepUntil time.Time
	for ; ; time.Sleep(time.Until(sleepUntil)) {
		b.logger.Println("Starting automatic image build cycle")
		startTime := time.Now()
		sleepUntil = startTime.Add(minInterval)
		if b.extendAutoRebuildImages(client, minInterval*2) {
			continue
		}
		for _, streamName := range b.listStreamsToAutoRebuild() {
			b.rebuildImage(client, streamName, minInterval*2)
		}
		b.logger.Printf("Completed automatic image build cycle in %s\n",
			format.Duration(time.Since(startTime)))
	}
}

func (err *buildErrorType) Error() string {
	return err.error
}

func (bl *dualBuildLogger) Bytes() []byte {
	return bl.buffer.Bytes()
}

func (bl *dualBuildLogger) Write(p []byte) (int, error) {
	return bl.writer.Write(p)
}
