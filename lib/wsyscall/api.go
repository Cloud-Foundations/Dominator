package wsyscall

import "syscall"

const (
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

type Timeval struct {
	Sec  int64
	Usec int64
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
	return setMyPriority(priority)
}

// SetNetNamespace is a safe wrapper for the Linux setns(fd, CLONE_NEWNET)
// system call. On Linux it will lock the current goroutine to an OS thread and
// set the network namespace to the specified file descriptor. On failure or on
// other platforms an error is returned.
func SetNetNamespace(fd int) error {
	return setNetNamespace(fd)
}

func Stat(path string, statbuf *Stat_t) error {
	return stat(path, statbuf)
}

func Sync() error {
	return sync()
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
