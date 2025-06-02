package fsutil

import (
	"errors"
	"hash"
	"io"
	"os"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

const (
	DirPerms = wsyscall.S_IRWXU | wsyscall.S_IRGRP | wsyscall.S_IXGRP |
		wsyscall.S_IROTH | wsyscall.S_IXOTH
	PrivateDirPerms  = wsyscall.S_IRWXU
	PrivateFilePerms = wsyscall.S_IRUSR | wsyscall.S_IWUSR
	PublicFilePerms  = PrivateFilePerms | wsyscall.S_IRGRP | wsyscall.S_IROTH
)

var (
	ErrorChecksumMismatch = errors.New("checksum mismatch")
)

// CompareFile will read and compare the content of a file and buffer and will
// return true if the contents are the same else false.
func CompareFile(buffer []byte, filename string) (bool, error) {
	return compareFile(buffer, filename)
}

// CompareFiles will read and compare the content of two files and return true
// if they are the same else false.
func CompareFiles(leftFilename, rightFilename string) (bool, error) {
	return compareFiles(leftFilename, rightFilename)
}

// CopyFile will create a new file, copies data from the sourceFilename to a
// tmpfile and then atomically renames the tmpfile to destFilename, ensuring
// that the file never has incomplete data.
// If there are any errors, then destFilename is unchanged.
// CopyFile is not safe to call concurrently for the same file.
func CopyFile(destFilename, sourceFilename string, mode os.FileMode) error {
	return copyFile(destFilename, sourceFilename, mode, false)
}

// CopyFileExclusive will exclusively create a new file, copies data from the
// sourceFilename to a tmpfile and then atomically moves the tmpfile to
// destFilename, ensuring that the file never has incomplete data.
// If there are any errors, then destFilename is unchanged.
func CopyFileExclusive(destFilename, sourceFilename string,
	mode os.FileMode) error {
	return copyFile(destFilename, sourceFilename, mode, true)
}

// CopyToFile will create a new file, write length bytes from reader to a
// tmpfile and then atomically renames the tmpfile to destFilename, ensuring
// that the file never has incomplete data.
// If length is zero all remaining bytes from reader are written. If there are
// any errors, then destFilename is unchanged.
// CopyToFile is not safe to call concurrently for the same file.
func CopyToFile(destFilename string, perm os.FileMode, reader io.Reader,
	length uint64) error {
	return copyToFile(destFilename, perm, reader, length)
}

// CopyToFileExclusive will exclusively create a new file, write length bytes
// from reader to a tmpfile and then atomically moves the tmpfile to
// destFilename, ensuring that the file never has incomplete data.
// If length is zero all remaining bytes from reader are written. If there are
// any errors, then destFilename is unchanged.
func CopyToFileExclusive(destFilename string, perm os.FileMode,
	reader io.Reader, length uint64) error {
	return copyToFileExclusive(destFilename, perm, reader, length)
}

// CopyTree will copy a directory tree.
func CopyTree(destDir, sourceDir string) error {
	return copyTree(destDir, sourceDir, true, CopyFile)
}

// CopyFilesTree will copy a directory tree of regular files. Other inode types
// are ignored.
func CopyFilesTree(destDir, sourceDir string) error {
	return copyTree(destDir, sourceDir, false, CopyFile)
}

// CopyTreeWithCopyFunc is similar to CopyTree except it uses a specified copy
// function for copying regular files.
func CopyTreeWithCopyFunc(destDir, sourceDir string,
	copyFunc func(destFilename, sourceFilename string,
		mode os.FileMode) error) error {
	return copyTree(destDir, sourceDir, true, copyFunc)
}

// Fallocate will allocate blocks for the file named filename, up to size
// specified in bytes.
func Fallocate(filename string, size uint64) error {
	return fallocate(filename, size)
}

// FallocateOrFill will allocate blocks for the file named filename, up to size
// specified in bytes. If allocation is not supported/available, the file will
// be filled with zeros (and a debug message will be logged).
func FallocateOrFill(filename string, size uint64,
	logger log.DebugLogger) error {
	return fallocateOrFill(filename, size, logger)
}

// ForceLink creates newname as a hard link to the oldname file. It first
// attempts to link using os.Link. If the first attempt fails due to
// a permission error, it blindly calls MakeMutable and then retries. If the
// first attempt fails due to newname existing, it blindly removes it and then
// retries.
func ForceLink(oldname, newname string) error {
	return forceLink(oldname, newname)
}

// ForceRemove removes the named file or directory. It first attempts to remove
// using os.Remove and that fails, it blindly calls MakeMutable and then
// retries.
func ForceRemove(name string) error {
	return forceRemove(name)
}

// ForceRemoveAll removes path and any children it contains. It first attempts
// to remove using os.RemoveAll and that fails, it blindly calls MakeMutable and
// then retries.
func ForceRemoveAll(path string) error {
	return forceRemoveAll(path)
}

// ForceRename renames (moves) a file. It first attempts to rename using
// os.Rename and if that fails due to a permission error, it blindly calls
// MakeMutable and then retries. If it fails because newpath is a directory, it
// calls ForceRemoveAll(newpath) and tries again.
func ForceRename(oldpath, newpath string) error {
	return forceRename(oldpath, newpath)
}

// FsyncFile will call file.Sync if it has not been called recently. This
// attempts to reduce the performance problems of fsync(2) by potentially
// sacrificing some file-system consistency.
func FsyncFile(file *os.File) error { return fsyncFile(file) }

// GetTreeSize will walk a directory tree and count the size of the files.
func GetTreeSize(dirname string) (uint64, error) {
	return getTreeSize(dirname)
}

// LoadLines will open a file and read lines from it. Comment lines (i.e. lines
// beginning with '#') are skipped.
func LoadLines(filename string) ([]string, error) {
	return loadLines(filename)
}

// LoopbackDelete will disassociate (delete) a loopback block device from its
// backing file.
func LoopbackDelete(loopDevice string) error {
	return loopbackDelete(loopDevice, true)
}

// LoopbackDeleteAndWaitForPartition will disassociate (delete) a loopback block
// device from its backing file and wait for specified partition inode to
// disappear.
func LoopbackDeleteAndWaitForPartition(loopDevice, partition string,
	timeout time.Duration, logger log.DebugLogger) error {
	return loopbackDeleteAndWaitForPartition(loopDevice, partition, timeout,
		logger)
}

// LoopbackSetup will associate a loopback block device with a regular file
// named filename. The name of the loop block device is returned.
func LoopbackSetup(filename string) (string, error) {
	return loopbackSetup(filename, true)
}

// LoopbackSetupAndWaitForPartition will associate a loopback block device with
// a regular file named filename and wait for the specified partition block
// device node to become available. The timeout is limited to one hour.
// The name of the loop block device (excluding the partition) is returned.
func LoopbackSetupAndWaitForPartition(filename, partition string,
	timeout time.Duration, logger log.DebugLogger) (string, error) {
	return loopbackSetupAndWaitForPartition(filename, partition, timeout,
		logger)
}

// MakeMutable attempts to remove the "immutable" and "append-only" ext2
// file-system attributes for one or more files. It is equivalent to calling the
// command-line programme "chattr -ai pathname...".
func MakeMutable(pathname ...string) error {
	return makeMutable(pathname...)
}

// ReadDirnames will open the directory named dirname and will read the entries
// in that directory. If ignoreMissing is true, no error is returned if the
// directory does not exist.
func ReadDirnames(dirname string, ignoreMissing bool) ([]string, error) {
	return readDirnames(dirname, ignoreMissing)
}

// ReadFileTree will traverse the specified directory tree rooted at topdir and
// reads the content of each file. The data are returned in a map with the keys
// being the filename (excluding the topdir and the specified prefix) and the
// data being the corresponding file contents.
func ReadFileTree(topdir, prefix string) (map[string][]byte, error) {
	return readFileTree(topdir, prefix)
}

// ReadLines will read lines from a reader. Comment lines (i.e. lines beginning
// with '#') are skipped.
func ReadLines(reader io.Reader) ([]string, error) {
	return readLines(reader)
}

// UpdateFile will read and compare the contents of a file and buffer and will
// update the file if different. It returns true if the contents were updated.
func UpdateFile(buffer []byte, filename string) (bool, error) {
	return updateFile(buffer, filename)
}

// WaitFile waits for the file given by pathname to become available to read and
// yields a io.ReadCloser when available, or an error if the timeout is
// exceeded or an error (other than file not existing) is encountered. A
// negative timeout indicates to wait forever. The io.ReadCloser must be closed
// after use.
func WaitFile(pathname string, timeout time.Duration) (io.ReadCloser, error) {
	return waitFile(pathname, timeout)
}

// WaitForBlockAvailable will wait for the specified block device node to
// become available, or return an error on timeout. The timeout is limited to
// one hour. The number of iterations and the number of successful Open(2) calls
// is returned.
// This is needed in enviroments where block devices such as partitions are
// dynamically created and there is a delay from creation to actual
// availability.
func WaitForBlockAvailable(pathname string,
	timeout time.Duration) (uint, uint, error) {
	return waitForBlockAvailable(pathname, timeout)
}

// WatchFile watches the file given by pathname and yields a new io.ReadCloser
// when a new inode is found and it is a regular file. The io.ReadCloser must
// be closed after use.
// Any errors are logged to the logger if it is not nil.
func WatchFile(pathname string, logger log.Logger) <-chan io.ReadCloser {
	return watchFile(pathname, logger)
}

// WatchFileStop stops all file watching and cleans up resources that would
// otherwise persist across syscall.Exec.
func WatchFileStop() {
	watchFileStop()
}

// ChecksumReader uses the SHA512 hash to checksum data as they are read.
type ChecksumReader struct {
	checksummer hash.Hash
	reader      io.Reader
}

// NewChecksumReader creates a ChecksumReader using reader as the underlying
// source of data. Checksumming is enabled by default.
func NewChecksumReader(reader io.Reader) *ChecksumReader {
	return newChecksumReader(reader)
}

// EnableChecksumming enables checksumming if enable is true and clears any
// prior checksum data, else disables checksumming.
func (r *ChecksumReader) EnableChecksumming(enable bool) {
	r.enableChecksumming(enable)
}

// GetChecksum returns the checksum of the data read so far. It returns nil if
// checksumming is disabled.
func (r *ChecksumReader) GetChecksum() []byte {
	return r.getChecksum()
}

// Read will read data into the buffer p.
func (r *ChecksumReader) Read(p []byte) (int, error) {
	return r.read(p)
}

// ReadByte will read a single byte.
func (r *ChecksumReader) ReadByte() (byte, error) {
	return r.readByte()
}

// VerifyChecksum will read a checksum from the underlying reader and will
// verify it matches the checksum that GetChecksum would return.
func (r *ChecksumReader) VerifyChecksum() error {
	return r.verifyChecksum()
}

// ChecksumWriter uses the SHA512 hash to checksum data as they are written.
type ChecksumWriter struct {
	checksummer hash.Hash
	writer      io.Writer
}

// NewChecksumWriter creates a ChecksumWriter using writer as the underlying
// destination for data. Checksumming is enabled by default.
func NewChecksumWriter(writer io.Writer) *ChecksumWriter {
	return newChecksumWriter(writer)
}

// EnableChecksumming enables checksumming if enable is true and clears any
// prior checksum data, else disables checksumming.
func (w *ChecksumWriter) EnableChecksumming(enable bool) {
	w.enableChecksumming(enable)
}

// GetChecksum returns the checksum of the data written so far. It returns nil
// if checksumming is disabled.
func (w *ChecksumWriter) GetChecksum() []byte {
	return w.getChecksum()
}

// Write will write the data specified by p.
func (w *ChecksumWriter) Write(p []byte) (int, error) {
	return w.write(p)
}

// WriteChecksum will write the checksum for the data written so far and will
// write it to the underlying writer.
func (w *ChecksumWriter) WriteChecksum() error {
	return w.writeChecksum()
}

// RenamingWriter is similar to a writable os.File, except that it attempts to
// ensure data integrity. A temporary file is used for writing, which is
// renamed during the Close method and an fsync(2) is attempted.
type RenamingWriter struct {
	*os.File
	filename string
	abort    bool
}

// CreateRenamingWriter will create a temporary file for writing and will rename
// the temporary file to filename in the Close method if there are no write
// errors.
func CreateRenamingWriter(filename string, perm os.FileMode) (
	*RenamingWriter, error) {
	return createRenamingWriter(filename, perm)
}

// Abort will prevent file renaming during a subsequent Close method call.
func (w *RenamingWriter) Abort() {
	w.abort = true
}

// Close may attempt to fsync(2) the contents of the temporary file (if fsync(2)
// has not been called recently) and will then close and rename the temporary
// file if there were no Write errors or a call to the Abort method.
func (w *RenamingWriter) Close() error {
	if err := recover(); err != nil {
		w.abort = true
		w.close()
		panic(err)
	}
	return w.close()
}

func (w *RenamingWriter) Write(p []byte) (int, error) {
	return w.write(p)
}
