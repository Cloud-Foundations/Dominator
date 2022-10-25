package logbuf

import (
	"fmt"
)

func localDup(oldfd int, newfd int) error {
	return fmt.Errorf("dup not implemented")
}
