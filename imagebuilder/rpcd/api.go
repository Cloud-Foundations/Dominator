package rpcd

import (
	"flag"
	"io"

	"github.com/Cloud-Foundations/Dominator/imagebuilder/builder"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/srpc/serverutil"
)

var (
	maximumConcurrentBuildsPerUser = flag.Uint("maximumConcurrentBuildsPerUser",
		1, "Maximum number of concurrent builds per unprivileged user")
)

type srpcType struct {
	builder *builder.Builder
	logger  log.Logger
	*serverutil.PerUserMethodLimiter
}

type htmlWriter srpcType

func (hw *htmlWriter) WriteHtml(writer io.Writer) {
	hw.writeHtml(writer)
}

func Setup(builder *builder.Builder, logger log.Logger) (*htmlWriter, error) {
	srpcObj := &srpcType{
		builder: builder,
		logger:  logger,
		PerUserMethodLimiter: serverutil.NewPerUserMethodLimiter(
			map[string]uint{
				"BuildImage": *maximumConcurrentBuildsPerUser,
			}),
	}
	srpc.RegisterNameWithOptions("Imaginator", srpcObj,
		srpc.ReceiverOptions{
			PublicMethods: []string{
				"BuildImage",
				"GetDependencies",
				"GetDirectedGraph",
			}})
	return (*htmlWriter)(srpcObj), nil
}
