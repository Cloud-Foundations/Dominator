package packageutil

import (
	"io"

	"github.com/Cloud-Foundations/Dominator/lib/image"
)

// GetPackageList will get the list of packages using the specified packager
// function.
// The packager function must support the "list" and "show-size-multiplier"
// commands.
func GetPackageList(packager func(cmd string, w io.Writer) error) (
	[]image.Package, error) {
	return getPackageList(packager)
}
