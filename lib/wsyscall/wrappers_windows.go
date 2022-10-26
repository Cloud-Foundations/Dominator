package wsyscall

import "syscall"

const (
	S_IFBLK  = syscall.S_IFBLK
	S_IFCHR  = syscall.S_IFCHR
	S_IFDIR  = syscall.S_IFDIR
	S_IFIFO  = syscall.S_IFIFO
	S_IFLNK  = syscall.S_IFLNK
	S_IFMT   = syscall.S_IFMT
	S_IFREG  = syscall.S_IFREG
	S_IFSOCK = syscall.S_IFSOCK
	S_IREAD  = syscall.S_IRUSR
	S_IRGRP  = 0x20
	S_IROTH  = 0x4
	S_IRUSR  = syscall.S_IRUSR
	S_IRWXG  = 0x38
	S_IRWXO  = 0x7
	S_IRWXU  = 0x1c0
	S_ISGID  = syscall.S_ISGID
	S_ISUID  = syscall.S_ISUID
	S_ISVTX  = syscall.S_ISVTX
	S_IWGRP  = 0x10
	S_IWOTH  = 0x2
	S_IWRITE = syscall.S_IWRITE
	S_IWUSR  = syscall.S_IWUSR
	S_IXGRP  = 0x8
	S_IXOTH  = 0x1
	S_IXUSR  = syscall.S_IXUSR
)

func fallocate(fd int, mode uint32, off int64, len int64) error {
	return syscall.ENOTSUP
}

func getFileDescriptorLimit() (uint64, uint64, error) {
	return 0, 0, syscall.ENOTSUP
}

func getrusage(who int, rusage *Rusage) error {
	return syscall.ENOTSUP
}

func ioctl(fd int, request, argp uintptr) error {
	return syscall.ENOTSUP
}

func lstat(path string, statbuf *Stat_t) error {
	return syscall.ENOTSUP
}

func mount(source string, target string, fstype string, flags uintptr,
	data string) error {
	return syscall.ENOTSUP
}

func reboot() error {
	return syscall.ENOTSUP
}

func setAllGid(gid int) error {
	return syscall.ENOTSUP
}

func setAllUid(uid int) error {
	return syscall.ENOTSUP
}

func setMyPriority(priority int) error {
	return syscall.ENOTSUP
}

func setNetNamespace(namespaceFd int) error {
	return syscall.ENOTSUP
}

func stat(path string, statbuf *Stat_t) error {
	return syscall.ENOTSUP
}

func sync() error {
	return syscall.ENOTSUP
}

func unshareNetNamespace() (int, int, error) {
	return -1, -1, syscall.ENOTSUP
}

func unshareMountNamespace() error {
	return syscall.ENOTSUP
}
