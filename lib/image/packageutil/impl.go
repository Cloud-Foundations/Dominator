package packageutil

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/Cloud-Foundations/Dominator/lib/image"
)

func getPackageList(packager func(cmd string, w io.Writer) error) (
	[]image.Package, error) {
	output := new(bytes.Buffer)
	if err := packager("show-size-multiplier", output); err != nil {
		return nil, fmt.Errorf("error getting size multiplier: %s", err)
	}
	sizeMultiplier := uint64(1)
	nScanned, err := fmt.Fscanf(output, "%d", &sizeMultiplier)
	if err != nil {
		if err != io.EOF {
			return nil, fmt.Errorf(
				"error decoding size multiplier: %s", err)
		}
	} else if nScanned != 1 {
		return nil, errors.New("malformed size multiplier")
	}
	output.Reset()
	if err := packager("list", output); err != nil {
		return nil, fmt.Errorf("error running package lister: %s", err)
	}
	packageMap := make(map[string]image.Package)
	scanner := bufio.NewScanner(output)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return nil, fmt.Errorf("malformed line: %s", line)
		}
		name := fields[0]
		version := fields[1]
		pkg := image.Package{
			Name:    name,
			Version: version,
		}
		if len(fields) > 2 {
			if size, err := strconv.ParseUint(fields[2], 10, 64); err != nil {
				return nil, fmt.Errorf("malformed size: %s", fields[2])
			} else {
				pkg.Size = size * sizeMultiplier
			}
		}
		packageMap[name] = pkg
	}
	if err := scanner.Err(); err != nil {
		if err != io.EOF {
			fmt.Fprintln(os.Stderr, "reading standard input:", err)
		}
	}
	packageNames := make([]string, 0, len(packageMap))
	for name := range packageMap {
		packageNames = append(packageNames, name)
	}
	sort.Strings(packageNames)
	var packages []image.Package
	for _, name := range packageNames {
		packages = append(packages, packageMap[name])
	}
	return packages, nil
}
