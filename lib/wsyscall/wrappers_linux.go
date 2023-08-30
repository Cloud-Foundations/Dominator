//go:build go1.11.6 || go1.12
// +build go1.11.6 go1.12

// Go versions prior to 1.10 would re-use a thread that was locked to a
// goroutine that exited. While go1.10 prevented thread re-use, it wasn't until
// go1.11.6/go1.12 that this was reliable:
// https://github.com/golang/go/issues/28979

package wsyscall

import (
	"errors"
	"os"
	"runtime"
	"strconv"
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

	sys_SETNS = 308 // 64 bit only.
)

func convertStat(dest *Stat_t, source *syscall.Stat_t) {
	dest.Dev = source.Dev
	dest.Ino = source.Ino
	dest.Nlink = uint64(source.Nlink)
	dest.Mode = source.Mode
	dest.Uid = source.Uid
	dest.Gid = source.Gid
	dest.Rdev = source.Rdev
	dest.Size = source.Size
	dest.Blksize = int64(source.Blksize)
	dest.Blocks = source.Blocks
	dest.Atim = source.Atim
	dest.Mtim = source.Mtim
	dest.Ctim = source.Ctim
}

func dup(oldfd int) (int, error) {
	return syscall.Dup(oldfd)
}

// Arm64 linux does NOT support the Dup2 syscall
// https://marcin.juszkiewicz.com.pl/download/tables/syscalls.html
// and dup3 is more supported so doing it here:
func dup2(oldfd int, newfd int) error {
	return syscall.Dup3(oldfd, newfd, 0)
}

func dup3(oldfd int, newfd int, flags int) error {
	return syscall.Dup3(oldfd, newfd, flags)
}

func fallocate(fd int, mode uint32, off int64, len int64) error {
	return syscall.Fallocate(fd, mode, off, len)
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
	case RUSAGE_THREAD:
		who = syscall.RUSAGE_THREAD
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
	rusage.Maxrss = int64(syscallRusage.Maxrss)
	rusage.Ixrss = int64(syscallRusage.Ixrss)
	rusage.Idrss = int64(syscallRusage.Idrss)
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
	var linuxFlags uintptr
	if flags&MS_BIND != 0 {
		linuxFlags |= syscall.MS_BIND
	}
	if flags&MS_RDONLY != 0 {
		linuxFlags |= syscall.MS_RDONLY
	}
	return syscall.Mount(source, target, fstype, linuxFlags, data)
}

func reboot() error {
	return syscall.Reboot(syscall.LINUX_REBOOT_CMD_RESTART)
}

func setAllGid(gid int) error {
	return syscall.Setresgid(gid, gid, gid)
}

func setAllUid(uid int) error {
	return syscall.Setresuid(uid, uid, uid)
}

// setMyPriority sets the priority of the current process, for all OS threads.
// It will iterate over all the threads and set the priority on each, since the
// Linux implementation of setpriority(2) only applies to a thread, not the
// whole process (contrary to the POSIX specification).
func setMyPriority(priority int) error {
	file, err := os.Open("/proc/self/task")
	if err != nil {
		return err
	}
	defer file.Close()
	taskNames, err := file.Readdirnames(0)
	if err != nil {
		return err
	}
	for _, taskName := range taskNames {
		taskId, err := strconv.Atoi(taskName)
		if err != nil {
			return err
		}
		err = syscall.Setpriority(syscall.PRIO_PROCESS, taskId, priority)
		if err != nil {
			return err
		}
	}
	return nil
}

func setNetNamespace(namespaceFd int) error {
	runtime.LockOSThread()
	_, _, errno := syscall.Syscall(sys_SETNS, uintptr(namespaceFd),
		uintptr(syscall.CLONE_NEWNET), 0)
	if errno != 0 {
		return os.NewSyscallError("setns", errno)
	}
	return nil

}

func stat(path string, statbuf *Stat_t) error {
	var rawStatbuf syscall.Stat_t
	if err := syscall.Stat(path, &rawStatbuf); err != nil {
		return err
	}
	convertStat(statbuf, &rawStatbuf)
	return nil
}

func unshareMountNamespace() error {
	// Pin goroutine to OS thread. This hack is required because
	// syscall.Unshare() operates on only one thread in the process, and Go
	// switches execution between threads randomly. Thus, the namespace can be
	// suddenly switched for running code. This is an aspect of Go that was not
	// well thought out.
	runtime.LockOSThread()
	if err := syscall.Unshare(syscall.CLONE_NEWNS); err != nil {
		return errors.New("error unsharing mount namespace: " + err.Error())
	}
	err := syscall.Mount("none", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, "")
	if err != nil {
		return errors.New("error making mounts private: " + err.Error())
	}
	return nil
}

func sync() error {
	syscall.Sync()
	return nil
}

func unshareNetNamespace() (int, int, error) {
	runtime.LockOSThread()
	if err := syscall.Unshare(syscall.CLONE_NEWNET); err != nil {
		return -1, -1,
			errors.New("error unsharing net namespace: " + err.Error())
	}
	tid := syscall.Gettid()
	tidString := strconv.FormatInt(int64(tid), 10)
	fd, err := syscall.Open("/proc/"+tidString+"/ns/net", syscall.O_RDONLY, 0)
	if err != nil {
		return -1, -1,
			errors.New("error getting FD for namespace: " + err.Error())
	}
	return fd, tid, nil
}
