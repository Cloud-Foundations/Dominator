package wsyscall

import "syscall"

const (
	FALLOC_FL_COLLAPSE_RANGE = 0x8
	FALLOC_FL_INSERT_RANGE   = 0x20
	FALLOC_FL_KEEP_SIZE      = 0x1
	FALLOC_FL_NO_HIDE_STALE  = 0x4
	FALLOC_FL_PUNCH_HOLE     = 0x2
	FALLOC_FL_UNSHARE_RANGE  = 0x40
	FALLOC_FL_ZERO_RANGE     = 0x10

	MS_BIND = 1 << iota
	MS_RDONLY

	RUSAGE_CHILDREN = iota
	RUSAGE_SELF
	RUSAGE_THREAD
)

type Rusage struct {
	Utime    Timeval
	Stime    Timeval
	Maxrss   int64
	Ixrss    int64
	Idrss    int64
	Isrss    int64
	Minflt   int64
	Majflt   int64
	Nswap    int64
	Inblock  int64
	Oublock  int64
	Msgsnd   int64
	Msgrcv   int64
	Nsignals int64
	Nvcsw    int64
	Nivcsw   int64
}

type Stat_t struct {
	Dev     uint64
	Ino     uint64
	Nlink   uint64
	Mode    uint32
	Uid     uint32
	Gid     uint32
	Rdev    uint64
	Size    int64
	Blksize int64
	Blocks  int64
	Atim    syscall.Timespec
	Mtim    syscall.Timespec
	Ctim    syscall.Timespec
}

type Statfs_t struct {
	Type   uint64
	Bsize  uint64
	Blocks uint64
	Bfree  uint64
	Bavail uint64
	Files  uint64
	Ffree  uint64
}

type Timeval struct {
	Sec  int64
	Usec int64
}

// ConvertStat will convert a *syscall.Stat_t to a *Stat_t. It returns an error
// if buf is not of type *syscall.Stat_t.
func ConvertStat(dest *Stat_t, source any) error {
	return convertStatAny(dest, source)
}

func Dup(oldfd int) (int, error) {
	return dup(oldfd)
}

func Dup2(oldfd int, newfd int) error {
	return dup2(oldfd, newfd)
}

func Dup3(oldfd int, newfd int, flags int) error {
	return dup3(oldfd, newfd, flags)
}

func Fallocate(fd int, mode uint32, off int64, len int64) error {
	return fallocate(fd, mode, off, len)
}

func Fstat(fd int, stat *Stat_t) error {
	return fstat(fd, stat)
}

func GetDeviceSize(device string) (uint64, error) {
	return getDeviceSize(device)
}

// GetFileDescriptorLimit returns the current limit and maximum limit on number
// of open file descriptors.
func GetFileDescriptorLimit() (uint64, uint64, error) {
	return getFileDescriptorLimit()
}

func Ioctl(fd int, request, argp uintptr) error {
	return ioctl(fd, request, argp)
}

func Lstat(path string, statbuf *Stat_t) error {
	return lstat(path, statbuf)
}

func Mkfifo(path string, mode uint32) error {
	return mkfifo(path, mode)
}

func Mknod(path string, mode uint32, dev int) error {
	return mknod(path, mode, dev)
}

func Mount(source string, target string, fstype string, flags uintptr,
	data string) error {
	return mount(source, target, fstype, flags, data)
}

func Getrusage(who int, rusage *Rusage) error {
	return getrusage(who, rusage)
}

func Reboot() error {
	return reboot()
}

func SetAllGid(gid int) error {
	return setAllGid(gid)
}

func SetAllUid(uid int) error {
	return setAllUid(uid)
}

// SetMyPriority sets the priority of the current process, for all OS threads.
// On platforms which do not support changing the process priority, an error is
// always returned.
func SetMyPriority(priority int) error {
	return setPriority(0, priority)
}

// SetNetNamespace is a safe wrapper for the Linux setns(fd, CLONE_NEWNET)
// system call. On Linux it will lock the current goroutine to an OS thread and
// set the network namespace to the specified file descriptor. On failure or on
// other platforms an error is returned.
func SetNetNamespace(fd int) error {
	return setNetNamespace(fd)
}

// SetPriority sets the CPU priority of the specified process, for all OS
// threads. If pid is zero, the priority of the calling process is set.
// On platforms which do not support changing the process priority, an error is
// always returned.
func SetPriority(pid, priority int) error {
	return setPriority(pid, priority)
}

// SetSysProcAttrChroot sets the Chroot field in attr. It returns an error if
// this operation is not supported.
func SetSysProcAttrChroot(attr *syscall.SysProcAttr, chroot string) error {
	return setSysProcAttrChroot(attr, chroot)
}

func Stat(path string, statbuf *Stat_t) error {
	return stat(path, statbuf)
}

func Statfs(path string, buf *Statfs_t) error {
	return statfs(path, buf)
}

func Sync() error {
	return sync()
}

func Unmount(target string, flags int) error {
	return unmount(target, flags)
}

// UnshareMountNamespace is a safe wrapper for the Linux unshare(CLONE_NEWNS)
// system call. On Linux it will lock the current goroutine to an OS thread and
// unshare the mount namespace (the thread will have a private copy of the
// previous mount namespace). On failure or on other platforms an error is
// returned.
func UnshareMountNamespace() error {
	return unshareMountNamespace()
}

// UnshareNetNamespace is a safe wrapper for the Linux unshare(CLONE_NEWNET)
// system call. On Linux it will lock the current goroutine to an OS thread and
// unshare the network namespace (the thread will have a fresh network
// namespace). The file descriptor for the new network namespace and the Linux
// thread ID are returned.
// On failure or on other platforms an error is returned.
func UnshareNetNamespace() (fd int, tid int, err error) {
	return unshareNetNamespace()
}
