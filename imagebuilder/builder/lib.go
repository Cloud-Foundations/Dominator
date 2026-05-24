package builder

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/types"
)

type fileCleanType struct {
	modTime  time.Time
	pathname string
	size     types.Bytes
}

func (b *Builder) replaceIdleSlaves(immediateGetNew bool) error {
	if b.slaveDriver == nil {
		return errors.New("no SlaveDriver configured")
	}
	b.slaveDriver.ReplaceIdle(immediateGetNew)
	return nil
}

func convertBindMounts(bindMounts []string) []bindMountType {
	fullBindMounts := make([]bindMountType, 0, len(bindMounts))
	for _, bindMount := range bindMounts {
		fullBindMounts = append(fullBindMounts, bindMountType{
			source: bindMount,
			target: bindMount,
		})
	}
	return fullBindMounts
}

func makeContext(deadline time.Duration) (context.Context, context.CancelFunc) {
	if deadline < time.Second {
		deadline = 24 * time.Hour
	}
	return context.WithDeadline(context.Background(), time.Now().Add(deadline))
}

func makeContext2(deadline0, deadline1 time.Duration) (
	context.Context, context.CancelFunc) {
	if deadline0 < time.Second {
		deadline0 = 24 * time.Hour
	}
	if deadline1 < time.Second {
		deadline1 = 24 * time.Hour
	}
	if deadline1 < deadline0 {
		deadline0 = deadline1
	}
	return context.WithDeadline(context.Background(), time.Now().Add(deadline0))
}

func trimDirectory(dirname string, sizeLimit types.Bytes,
	buildLog io.Writer) error {
	if sizeLimit < 1 {
		return nil // Unlimited: that's not a great idea.
	}
	var fileInfos []fileCleanType
	err := filepath.WalkDir(dirname,
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.Type()&fs.ModeType != 0 {
				return nil // Ignore anything that's not a regular file.
			}
			fi, err := d.Info()
			if err != nil {
				return err
			}
			fileInfos = append(fileInfos, fileCleanType{
				modTime:  fi.ModTime(),
				pathname: path,
				size:     types.Bytes(fi.Size()),
			})
			return nil
		})
	if err != nil {
		return err
	}
	var totalSize types.Bytes
	for _, file := range fileInfos {
		totalSize += file.size
	}
	sort.Slice(fileInfos, func(left, right int) bool {
		return fileInfos[left].modTime.Before(fileInfos[right].modTime)
	})
	for _, file := range fileInfos {
		if totalSize < sizeLimit {
			return nil
		}
		if err := os.Remove(file.pathname); err != nil {
			return err
		}
		fmt.Fprintf(buildLog, "Deleted: %s\n", file.pathname)
		totalSize -= file.size
	}
	return nil
}
