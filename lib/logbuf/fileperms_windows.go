package logbuf

import (
	"os"
)

const (
	dirPerms  = os.ModeDir | os.ModePerm
	filePerms = os.ModePerm
)
