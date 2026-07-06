package loadflags

func LoadForCli(progName string) error {
	return loadForCli(progName)
}

func LoadForDaemon(progName string) error {
	return loadForDaemon(progName)
}
