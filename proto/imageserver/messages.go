package imageserver

import (
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/tags"
)

type AddImageRequest struct {
	ImageName string
	Image     *image.Image
}

type AddImageResponse struct{}

type ChangeImageExpirationRequest struct {
	ExpiresAt time.Time
	ImageName string
}

type ChangeImageExpirationResponse struct {
	Error string
}

type ChangeOwnerRequest struct {
	DirectoryName string
	OwnerGroup    string
}

type ChangeOwnerResponse struct{}

type CheckDirectoryRequest struct {
	DirectoryName string
}

type CheckDirectoryResponse struct {
	DirectoryExists bool
}

type CheckImageRequest struct {
	ImageName string
}

type CheckImageResponse struct {
	ImageExists bool
}

type DeleteImageRequest struct {
	ImageName string
}

type DeleteImageResponse struct{}

type DeleteUnreferencedObjectsRequest struct {
	Percentage uint8
	Bytes      uint64
}

type DeleteUnreferencedObjectsResponse struct{}

type FindLatestImageRequest struct {
	BuildCommitId        string // Optional.
	DirectoryName        string
	IgnoreExpiringImages bool
	TagsToMatch          tags.MatchTags // Empty: match all tags.
}

type FindLatestImageResponse struct {
	ImageName string
	Error     string
}

type GetImageComputedFilesRequest struct {
	ImageName string
}

type GetImageComputedFilesResponse struct {
	ComputedFiles []filesystem.ComputedFile
	ImageExists   bool
}

type GetImageExpirationRequest struct {
	ImageName string
}

type GetImageExpirationResponse struct {
	Error     string
	ExpiresAt time.Time
}

type GetImageArchiveRequest struct {
	ImageName string
}

type GetImageArchiveResponse struct {
	ArchiveData       []byte // GOB encoding of ImageArchive followed by HMAC.
	Error             string
	ReplicationMaster string // If not empty, go here instead.
}

type GetImageRequest struct {
	ImageName                  string
	IgnoreFilesystem           bool
	IgnoreFilesystemIfExpiring bool
	Timeout                    time.Duration
}

type GetImageResponse struct {
	Image *image.Image
}

const (
	OperationAddImage      = 0
	OperationDeleteImage   = 1
	OperationMakeDirectory = 2
)

// The GetImageUpdates() RPC is fully streamed.
// The client sends no information to the server.
// The server sends a stream of ImageUpdate messages.

type GetImageUsageEstimateRequest struct {
	ImageName string
}

type GetImageUsageEstimateResponse struct {
	ImageExists   bool
	UsageEstimate uint64
}

// The GetFilteredImageUpdates() RPC is fully streamed.
// The client sends a GetFilteredImageUpdatesRequest message to the server.
// The server sends a stream of ImageUpdate messages.

type GetFilteredImageUpdatesRequest struct {
	IgnoreExpiring bool
}

type ImageUpdate struct {
	Name      string // "" signifies initial list is sent, changes to follow.
	Directory *image.Directory
	Operation uint
}

type GetReplicationMasterRequest struct{}

type GetReplicationMasterResponse struct {
	Error             string
	ReplicationMaster string
}

type ImageArchive struct {
	ImageName string
	image.Image
} // HMAC-SHA512 checksum is written after GOB encoded data.

// The ListDirectories() RPC is fully streamed.
// The client sends no information to the server.
// The server sends a stream of image.Directory values with an empty string
// for the Name field signifying the end of the list.

// The ListImages() RPC is fully streamed.
// The client sends no information to the server.
// The server sends a stream of strings (image names) with an empty string
// signifying the end of the list.

type ListSelectedImagesRequest struct {
	IgnoreExpiringImages bool
	TagsToMatch          tags.MatchTags // Empty: match all tags.
}

// The server sends a stream of strings (image names) with an empty string
// signifying the end of the list.

// The ListUnreferencedObjects() RPC is fully streamed.
// The client sends no information to the server.
// The server sends a stream of Object values with a zero Size field signifying
// the end of the stream.

type Object struct {
	Hash hash.Hash
	Size uint64
}

type MakeDirectoryRequest struct {
	DirectoryName string
	MakeAll       bool
}

type MakeDirectoryResponse struct{}

type RestoreImageFromArchiveRequest struct {
	ExpiresAt   time.Time
	ArchiveData []byte // GOB encoding of ImageArchive followed by HMAC.
}

type RestoreImageFromArchiveResponse struct {
	Error             string
	ReplicationMaster string // If not empty, go here instead.
}
