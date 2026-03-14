package storage

// SignedUrlRequest represents a request to get a signed URL
type SignedUrlRequest struct {
	Path   string `form:"path"`
	Expire int    `form:"expire"`
}

// SignedUrlResponse represents the response for getting a signed URL
type SignedUrlResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ListObjectsRequest represents a request to list objects
type ListObjectsRequest struct {
	Prefix          string `form:"prefix"`
	Delimiter       string `form:"delimiter"`
	Marker          string `form:"marker"`
	MaxKeys         int    `form:"maxKeys"`
	StorageClassURL string `form:"storageClassUrl"`
}

// ListObjectsResponse represents the response for listing objects
type ListObjectsResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// UploadObjectRequest represents a request to upload an object
type UploadObjectRequest struct {
	Path          string `form:"path"`
	ContentType   string `form:"contentType"`
	Content       string `form:"content"`
	StorageClass  string `form:"storageClass"`
	PreassignedID string `form:"preassignedId"`
}

// UploadObjectData represents the data for upload object response
type UploadObjectData struct {
	Path string `json:"path"`
}

// UploadObjectResponse represents the response for uploading an object
type UploadObjectResponse struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Data    UploadObjectData `json:"data"`
}

// DeleteObjectRequest represents a request to delete an object
type DeleteObjectRequest struct {
	Path string `form:"path"`
}

// DeleteObjectResponse represents the response for deleting an object
type DeleteObjectResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// BatchDeleteObjectsData represents the data for batch delete objects response
type BatchDeleteObjectsData struct {
	Deleted []string `json:"deleted"`
	Failed  []string `json:"failed"`
}

// BatchDeleteObjectsRequest represents a request to batch delete objects
type BatchDeleteObjectsRequest struct {
	Paths []string `json:"paths"`
}

// BatchDeleteObjectsResponse represents the response for batch deleting objects
type BatchDeleteObjectsResponse struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Data    BatchDeleteObjectsData `json:"data"`
}

// CreateDirectoryRequest represents a request to create a directory
type CreateDirectoryRequest struct {
	Prefix string `json:"prefix"`
}

// CreateDirectoryResponse represents the response for creating a directory
type CreateDirectoryResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// RenameDirectoryRequest represents a request to rename a directory
type RenameDirectoryRequest struct {
	OldPrefix string `json:"oldPrefix"`
	NewPrefix string `json:"newPrefix"`
}

// RenameDirectoryResponse represents the response for renaming a directory
type RenameDirectoryResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ObjectInfo represents information about a storage object
type ObjectInfo struct {
	Key          string `json:"key"`
	Size         int64  `json:"size"`
	LastModified string `json:"last_modified"`
	ETag         string `json:"etag"`
	StorageClass string `json:"storage_class"`
}

// ObjectsData represents the data returned when listing objects
type ObjectsData struct {
	Objects     []ObjectInfo `json:"objects"`
	Prefixes    []string     `json:"prefixes"`
	IsTruncated bool         `json:"is_truncated"`
	NextMarker  string       `json:"next_marker"`
}

// Type aliases for backward compatibility with types.CreateDirectoryRequest
// Deprecated: Use storage.CreateDirectoryRequest directly.
type CreateDirectoryRequestAlias = CreateDirectoryRequest

// Type aliases for backward compatibility with types.CreateDirectoryResponse
// Deprecated: Use storage.CreateDirectoryResponse directly.
type CreateDirectoryResponseAlias = CreateDirectoryResponse

// Type aliases for backward compatibility with types.DeleteObjectRequest
// Deprecated: Use storage.DeleteObjectRequest directly.
type DeleteObjectRequestAlias = DeleteObjectRequest

// Type aliases for backward compatibility with types.DeleteObjectResponse
// Deprecated: Use storage.DeleteObjectResponse directly.
type DeleteObjectResponseAlias = DeleteObjectResponse

// Type aliases for backward compatibility with types.ListObjectsRequest
// Deprecated: Use storage.ListObjectsRequest directly.
type ListObjectsRequestAlias = ListObjectsRequest

// Type aliases for backward compatibility with types.ListObjectsResponse
// Deprecated: Use storage.ListObjectsResponse directly.
type ListObjectsResponseAlias = ListObjectsResponse

// Type aliases for backward compatibility with types.ObjectInfo
// Deprecated: Use storage.ObjectInfo directly.
type ObjectInfoAlias = ObjectInfo

// Type aliases for backward compatibility with types.ObjectsData
// Deprecated: Use storage.ObjectsData directly.
type ObjectsDataAlias = ObjectsData

// Type aliases for backward compatibility with types.SignedUrlRequest
// Deprecated: Use storage.SignedUrlRequest directly.
type SignedUrlRequestAlias = SignedUrlRequest

// Type aliases for backward compatibility with types.SignedUrlResponse
// Deprecated: Use storage.SignedUrlResponse directly.
type SignedUrlResponseAlias = SignedUrlResponse

// Type aliases for backward compatibility with types.UploadObjectData
// Deprecated: Use storage.UploadObjectData directly.
type UploadObjectDataAlias = UploadObjectData

// Type aliases for backward compatibility with types.UploadObjectRequest
// Deprecated: Use storage.UploadObjectRequest directly.
type UploadObjectRequestAlias = UploadObjectRequest

// Type aliases for backward compatibility with types.UploadObjectResponse
// Deprecated: Use storage.UploadObjectResponse directly.
type UploadObjectResponseAlias = UploadObjectResponse
