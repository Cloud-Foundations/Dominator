package fsutil

import (
	"errors"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

func getFilePerms(filename string) (os.FileMode, error) {
	var stat wsyscall.Stat_t
	if err := wsyscall.Stat(filename, &stat); err != nil {
		return 0, errors.New(filename + ": " + err.Error())
	}
	mode := os.FileMode(stat.Mode) & os.ModePerm
	return mode, nil
}
