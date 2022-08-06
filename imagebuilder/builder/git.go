package builder

import (
	"io"
	stdlog "log"

	"github.com/Cloud-Foundations/Dominator/lib/gitutil"
	"github.com/Cloud-Foundations/Dominator/lib/log/debuglogger"
)

type writingLogger struct {
	*debuglogger.Logger
	io.Writer
}

func gitShallowClone(manifestRoot, manifestUrl, publicUrl, gitBranch string,
	patterns []string, buildLog io.Writer) error {
	logger := &writingLogger{
		Logger: debuglogger.New(stdlog.New(buildLog, "", 0)),
		Writer: buildLog,
	}
	logger.SetLevel(10)
	return gitutil.ShallowClone(manifestRoot, gitutil.ShallowCloneParams{
		GitBranch: gitBranch,
		Patterns:  patterns,
		PublicURL: publicUrl,
		RepoURL:   manifestUrl,
	},
		logger)
}
