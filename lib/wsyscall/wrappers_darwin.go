package wsyscall

import (
	"os"
	"syscall"
)

const (
	S_IFBLK  = syscall.S_IFBLK
	S_IFCHR  = syscall.S_IFCHR
	S_IFDIR  = syscall.S_IFDIR
	S_IFIFO  = syscall.S_IFIFO
	S_IFLNK  = syscall.S_IFLNK
	S_IFMT   = syscall.S_IFMT
	S_IFREG  = syscall.S_IFREG
	S_IFSOCK = syscall.S_IFSOCK
	S_IREAD  = syscall.S_IREAD
	S_IRGRP  = syscall.S_IRGRP
	S_IROTH  = syscall.S_IROTH
	S_IRUSR  = syscall.S_IRUSR
	S_IRWXG  = syscall.S_IRWXG
	S_IRWXO  = syscall.S_IRWXO
	S_IRWXU  = syscall.S_IRWXU
	S_ISGID  = syscall.S_ISGID
	S_ISUID  = syscall.S_ISUID
	S_ISVTX  = syscall.S_ISVTX
	S_IWGRP  = syscall.S_IWGRP
	S_IWOTH  = syscall.S_IWOTH
	S_IWRITE = syscall.S_IWRITE
	S_IWUSR  = syscall.S_IWUSR
	S_IXGRP  = syscall.S_IXGRP
	S_IXOTH  = syscall.S_IXOTH
	S_IXUSR  = syscall.S_IXUSR
)

func convertStat(dest *Stat_t, source *syscall.Stat_t) {
	dest.Dev = uint64(source.Dev)
	dest.Ino = source.Ino
	dest.Nlink = uint64(source.Nlink)
	dest.Mode = uint32(source.Mode)
	dest.Uid = source.Uid
	dest.Gid = source.Gid
	dest.Rdev = uint64(source.Rdev)
	dest.Size = source.Size
	dest.Blksize = int64(source.Blksize)
	dest.Blocks = source.Blocks
	dest.Atim = source.Atimespec
	dest.Mtim = source.Mtimespec
	dest.Ctim = source.Ctimespec
}

func dup(oldfd int) (int, error) {
	return syscall.Dup(oldfd)
}

func dup2(oldfd int, newfd int) error {
	return syscall.Dup2(oldfd, newfd)
}

func dup3(oldfd int, newfd int, flags int) error {
	if flags == 0 {
		return syscall.Dup2(oldfd, newfd)
	}
	return syscall.ENOTSUP
}

func getFileDescriptorLimit() (uint64, uint64, error) {
	var rlim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlim); err != nil {
		return 0, 0, err
	}
	return rlim.Cur, rlim.Max, nil
}

func getrusage(who int, rusage *Rusage) error {
	switch who {
	case RUSAGE_CHILDREN:
		who = syscall.RUSAGE_CHILDREN
	case RUSAGE_SELF:
		who = syscall.RUSAGE_SELF
	default:
		return syscall.ENOTSUP
	}
	var syscallRusage syscall.Rusage
	if err := syscall.Getrusage(who, &syscallRusage); err != nil {
		return err
	}
	rusage.Utime.Sec = int64(syscallRusage.Utime.Sec)
	rusage.Utime.Usec = int64(syscallRusage.Utime.Usec)
	rusage.Stime.Sec = int64(syscallRusage.Stime.Sec)
	rusage.Stime.Usec = int64(syscallRusage.Stime.Usec)
	rusage.Maxrss = int64(syscallRusage.Maxrss) >> 10
	rusage.Ixrss = int64(syscallRusage.Ixrss) >> 10
	rusage.Idrss = int64(syscallRusage.Idrss) >> 10
	rusage.Minflt = int64(syscallRusage.Minflt)
	rusage.Majflt = int64(syscallRusage.Majflt)
	rusage.Nswap = int64(syscallRusage.Nswap)
	rusage.Inblock = int64(syscallRusage.Inblock)
	rusage.Oublock = int64(syscallRusage.Oublock)
	rusage.Msgsnd = int64(syscallRusage.Msgsnd)
	rusage.Msgrcv = int64(syscallRusage.Msgrcv)
	rusage.Nsignals = int64(syscallRusage.Nsignals)
	rusage.Nvcsw = int64(syscallRusage.Nvcsw)
	rusage.Nivcsw = int64(syscallRusage.Nivcsw)
	return nil
}

func fallocate(fd int, mode uint32, off int64, len int64) error {
	return syscall.ENOTSUP
}

func ioctl(fd int, request, argp uintptr) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), request,
		argp)
	if errno != 0 {
		return os.NewSyscallError("ioctl", errno)
	}
	return nil
}

func lstat(path string, statbuf *Stat_t) error {
	var rawStatbuf syscall.Stat_t
	if err := syscall.Lstat(path, &rawStatbuf); err != nil {
		return err
	}
	convertStat(statbuf, &rawStatbuf)
	return nil
}

func mount(source string, target string, fstype string, flags uintptr,
	data string) error {
	return syscall.ENOTSUP
}

func reboot() error {
	return syscall.ENOTSUP
}

func setAllGid(gid int) error {
	return syscall.Setregid(gid, gid)
}

func setAllUid(uid int) error {
	return syscall.Setreuid(uid, uid)
}

func setMyPriority(priority int) error {
	return syscall.Setpriority(syscall.PRIO_PROCESS, 0, priority)
}

func setNetNamespace(namespaceFd int) error {
	return syscall.ENOTSUP
}

func stat(path string, statbuf *Stat_t) error {
	var rawStatbuf syscall.Stat_t
	if err := syscall.Stat(path, &rawStatbuf); err != nil {
		return err
	}
	convertStat(statbuf, &rawStatbuf)
	return nil
}

func sync() error {
	return syscall.Sync()
}

func unshareNetNamespace() (int, int, error) {
	return -1, -1, syscall.ENOTSUP
}

func unshareMountNamespace() error {
	return syscall.ENOTSUP
}
