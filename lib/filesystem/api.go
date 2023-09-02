package filesystem

import (
	"fmt"
	"io"

	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
)

type NumLinksTable map[uint64]int

type ListSelector uint8

const (
	ListSelectSkipMode = 1 << iota
	ListSelectSkipNumLinks
	ListSelectSkipUid
	ListSelectSkipGid
	ListSelectSkipSizeDevnum
	ListSelectSkipMtime
	ListSelectSkipName
	ListSelectSkipData

	ListSelectAll = 0
)

type GenericInode interface {
	GetGid() uint32
	GetUid() uint32
	List(w io.Writer, name string, numLinksTable NumLinksTable,
		numLinks int, listSelector ListSelector, filter *filter.Filter) error
	SetGid(gid uint32)
	SetUid(uid uint32)
	WriteMetadata(name string) error
}

type InodeTable map[uint64]GenericInode
type InodeToFilenamesTable map[uint64][]string
type FilenameToInodeTable map[string]uint64
type HashToInodesTable map[hash.Hash][]uint64

type FileSystem struct {
	InodeTable               InodeTable
	inodeToFilenamesTable    InodeToFilenamesTable
	filenameToInodeTable     FilenameToInodeTable
	hashToInodesTable        HashToInodesTable
	NumRegularInodes         uint64
	TotalDataBytes           uint64
	numComputedRegularInodes *uint64
	DirectoryCount           uint64
	DirectoryInode
}

func Decode(reader io.Reader) (*FileSystem, error) {
	return decode(reader)
}

func (fs *FileSystem) BuildNumLinksTable() NumLinksTable {
	return buildNumLinksTable(fs)
}

func (fs *FileSystem) ComputeTotalDataBytes() {
	fs.computeTotalDataBytes()
}

func (fs *FileSystem) Encode(writer io.Writer) error {
	return fs.encode(writer)
}

func (fs *FileSystem) EstimateUsage(blockSize uint64) uint64 {
	return fs.estimateUsage(blockSize)
}

func (fs *FileSystem) FilenameToInodeTable() FilenameToInodeTable {
	return fs.buildFilenameToInodeTable()
}

func (fs *FileSystem) Filter(filter *filter.Filter) *FileSystem {
	return fs.filter(filter)
}

func (fs *FileSystem) ForEachFile(
	fn func(name string, inodeNumber uint64, inode GenericInode) error) error {
	return fs.forEachFile(fn)
}

func (fs *FileSystem) GetObjects() map[hash.Hash]uint64 {
	return fs.getObjects()
}

func (fs *FileSystem) HashToInodesTable() HashToInodesTable {
	return fs.buildHashToInodesTable()
}

func (fs *FileSystem) InodeToFilenamesTable() InodeToFilenamesTable {
	return fs.buildInodeToFilenamesTable()
}

func (fs *FileSystem) List(w io.Writer) error {
	return fs.list(w, ListSelectAll, nil)
}

func (fs *FileSystem) Listf(w io.Writer, listSelector ListSelector,
	filter *filter.Filter) error {
	return fs.list(w, listSelector, filter)
}

func (fs *FileSystem) NumComputedRegularInodes() uint64 {
	return fs.computeNumComputedRegularInodes()
}

func (fs *FileSystem) RebuildInodePointers() error {
	return fs.rebuildInodePointers()
}

func (fs *FileSystem) String() string {
	return fmt.Sprintf("Tree: %d inodes, total file size: %s, number of regular inodes: %d",
		len(fs.InodeTable),
		format.FormatBytes(fs.TotalDataBytes),
		fs.NumRegularInodes)
}

type DirectoryInode struct {
	EntryList     []*DirectoryEntry
	EntriesByName map[string]*DirectoryEntry
	Mode          FileMode
	Uid           uint32
	Gid           uint32
}

func (directory *DirectoryInode) BuildEntryMap() {
	directory.buildEntryMap()
}

func (inode *DirectoryInode) GetGid() uint32 {
	return inode.Gid
}

func (inode *DirectoryInode) GetUid() uint32 {
	return inode.Uid
}

func (inode *DirectoryInode) List(w io.Writer, name string,
	numLinksTable NumLinksTable, numLinks int,
	listSelector ListSelector, filter *filter.Filter) error {
	return inode.list(w, name, numLinksTable, numLinks, listSelector, filter)
}

func (inode *DirectoryInode) RegisterStrings(registerFunc func(string)) {
	inode.registerStrings(registerFunc)
}

func (inode *DirectoryInode) ReplaceStrings(replaceFunc func(string) string) {
	inode.replaceStrings(replaceFunc)
}

func (inode *DirectoryInode) SetGid(gid uint32) {
	inode.Gid = gid
}

func (inode *DirectoryInode) SetUid(uid uint32) {
	inode.Uid = uid
}

func (inode *DirectoryInode) Write(name string) error {
	return inode.write(name)
}

func (inode *DirectoryInode) WriteMetadata(name string) error {
	return inode.writeMetadata(name)
}

type DirectoryEntry struct {
	Name        string
	InodeNumber uint64
	inode       GenericInode // Keep private to avoid encoding/transmission.
}

func (dirent *DirectoryEntry) Inode() GenericInode {
	return dirent.inode
}

func (dirent *DirectoryEntry) SetInode(inode GenericInode) {
	dirent.inode = inode
}

func (dirent *DirectoryEntry) String() string {
	return dirent.Name
}

type RegularInode struct {
	Mode             FileMode
	Uid              uint32
	Gid              uint32
	MtimeNanoSeconds int32
	MtimeSeconds     int64
	Size             uint64
	Hash             hash.Hash
}

func (inode *RegularInode) GetGid() uint32 {
	return inode.Gid
}

func (inode *RegularInode) GetUid() uint32 {
	return inode.Uid
}

func (inode *RegularInode) List(w io.Writer, name string,
	numLinksTable NumLinksTable, numLinks int,
	listSelector ListSelector, filter *filter.Filter) error {
	return inode.list(w, name, numLinksTable, numLinks, listSelector)
}

func (inode *RegularInode) SetGid(gid uint32) {
	inode.Gid = gid
}

func (inode *RegularInode) SetUid(uid uint32) {
	inode.Uid = uid
}

func (inode *RegularInode) WriteMetadata(name string) error {
	return inode.writeMetadata(name)
}

type ComputedRegularInode struct {
	Mode   FileMode
	Uid    uint32
	Gid    uint32
	Source string
}

func (inode *ComputedRegularInode) GetGid() uint32 {
	return inode.Gid
}

func (inode *ComputedRegularInode) GetUid() uint32 {
	return inode.Uid
}

func (inode *ComputedRegularInode) List(w io.Writer, name string,
	numLinksTable NumLinksTable, numLinks int,
	listSelector ListSelector, filter *filter.Filter) error {
	return inode.list(w, name, numLinksTable, numLinks, listSelector)
}

func (inode *ComputedRegularInode) SetGid(gid uint32) {
	inode.Gid = gid
}

func (inode *ComputedRegularInode) SetUid(uid uint32) {
	inode.Uid = uid
}

func (inode *ComputedRegularInode) WriteMetadata(name string) error {
	panic("cannot write metadata for computed file: " + name)
}

type SymlinkInode struct {
	Uid     uint32
	Gid     uint32
	Symlink string
}

func (inode *SymlinkInode) GetGid() uint32 {
	return inode.Gid
}

func (inode *SymlinkInode) GetUid() uint32 {
	return inode.Uid
}

func (inode *SymlinkInode) List(w io.Writer, name string,
	numLinksTable NumLinksTable, numLinks int,
	listSelector ListSelector, filter *filter.Filter) error {
	return inode.list(w, name, numLinksTable, numLinks, listSelector)
}

func (inode *SymlinkInode) SetGid(gid uint32) {
	inode.Gid = gid
}

func (inode *SymlinkInode) SetUid(uid uint32) {
	inode.Uid = uid
}

func (inode *SymlinkInode) Write(name string) error {
	return inode.write(name)
}

func (inode *SymlinkInode) WriteMetadata(name string) error {
	return inode.writeMetadata(name)
}

type SpecialInode struct {
	Mode             FileMode
	Uid              uint32
	Gid              uint32
	MtimeNanoSeconds int32
	MtimeSeconds     int64
	Rdev             uint64
}

func (inode *SpecialInode) GetGid() uint32 {
	return inode.Gid
}

func (inode *SpecialInode) GetUid() uint32 {
	return inode.Uid
}

func (inode *SpecialInode) List(w io.Writer, name string,
	numLinksTable NumLinksTable, numLinks int,
	listSelector ListSelector, filter *filter.Filter) error {
	return inode.list(w, name, numLinksTable, numLinks, listSelector)
}

func (inode *SpecialInode) SetGid(gid uint32) {
	inode.Gid = gid
}

func (inode *SpecialInode) SetUid(uid uint32) {
	inode.Uid = uid
}

func (inode *SpecialInode) Write(name string) error {
	return inode.write(name)
}

func (inode *SpecialInode) WriteMetadata(name string) error {
	return inode.writeMetadata(name)
}

type FileMode uint32

func (mode FileMode) String() string {
	return mode.string()
}

func CompareFileSystems(left, right *FileSystem, logWriter io.Writer) bool {
	return compareFileSystems(left, right, logWriter)
}

func CompareDirectoryInodes(left, right *DirectoryInode,
	logWriter io.Writer) bool {
	return compareDirectoryInodes(left, right, logWriter)
}

func CompareDirectoriesMetadata(left, right *DirectoryInode,
	logWriter io.Writer) bool {
	return compareDirectoriesMetadata(left, right, logWriter)
}

func CompareDirectoryEntries(left, right *DirectoryEntry,
	logWriter io.Writer) bool {
	return compareDirectoryEntries(left, right, logWriter)
}

func CompareInodes(left, right GenericInode, logWriter io.Writer) (
	sameType, sameMetadata, sameData bool) {
	return compareInodes(left, right, logWriter)
}

func CompareRegularInodes(left, right *RegularInode, logWriter io.Writer) bool {
	return compareRegularInodes(left, right, logWriter)
}

func CompareRegularInodesMetadata(left, right *RegularInode,
	logWriter io.Writer) bool {
	return compareRegularInodesMetadata(left, right, logWriter)
}

func CompareRegularInodesData(left, right *RegularInode,
	logWriter io.Writer) bool {
	return compareRegularInodesData(left, right, logWriter)
}

func CompareSymlinkInodes(left, right *SymlinkInode, logWriter io.Writer) bool {
	return compareSymlinkInodes(left, right, logWriter)
}

func CompareSymlinkInodesMetadata(left, right *SymlinkInode,
	logWriter io.Writer) bool {
	return compareSymlinkInodesMetadata(left, right, logWriter)
}

func CompareSymlinkInodesData(left, right *SymlinkInode,
	logWriter io.Writer) bool {
	return compareSymlinkInodesData(left, right, logWriter)
}

func CompareSpecialInodes(left, right *SpecialInode, logWriter io.Writer) bool {
	return compareSpecialInodes(left, right, logWriter)
}

func CompareSpecialInodesMetadata(left, right *SpecialInode,
	logWriter io.Writer) bool {
	return compareSpecialInodesMetadata(left, right, logWriter)
}

func CompareSpecialInodesData(left, right *SpecialInode,
	logWriter io.Writer) bool {
	return compareSpecialInodesData(left, right, logWriter)
}

func ForceWriteMetadata(inode GenericInode, name string) error {
	return forceWriteMetadata(inode, name)
}
