package logarchiver

import ()

type nullBuildLogArchiver struct{}

func newNullLogger() *nullBuildLogArchiver {
	return &nullBuildLogArchiver{}
}

func (a *nullBuildLogArchiver) AddBuildLog(imageName string,
	buildInfo BuildInfo, log []byte) error {
	return nil
}
