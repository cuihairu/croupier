package pack

// PacksListRequest represents a request to list packs
type PacksListRequest struct {
	Page     int `form:"page"`
	PageSize int `form:"pageSize"`
}

// PacksListResponse represents the response for listing packs
type PacksListResponse struct {
	Manifest           interface{} `json:"manifest,omitempty"`
	Packs              interface{} `json:"packs,omitempty"`
	Counts             interface{} `json:"counts,omitempty"`
	ETag               string      `json:"etag,omitempty"`
	ExportAuthRequired bool        `json:"exportAuthRequired,omitempty"`
}

// PacksExportRequest represents a request to export packs
type PacksExportRequest struct {
	ID string `form:"id"`
}

// PacksExportResponse represents the response for exporting packs
type PacksExportResponse struct {
	Filename    string `json:"filename,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	Content     []byte `json:"content,omitempty"`
}

// PacksImportRequest represents a request to import packs
type PacksImportRequest struct {
	Archive string `json:"archive"`
}

// PacksImportResponse represents the response for importing packs
type PacksImportResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// PacksReloadRequest represents a request to reload packs
type PacksReloadRequest struct{}

// PacksReloadResponse represents the response for reloading packs
type PacksReloadResponse struct {
	OK        bool   `json:"ok"`
	UpdatedAt string `json:"updatedAt"`
}

// PacksPluginRequest represents a request to get pack web plugin
type PacksPluginRequest struct {
	Pack string `form:"pack"`
	Path string `form:"path"`
}

// PacksPluginResponse represents the response for getting pack web plugin
type PacksPluginResponse struct {
	Content string `json:"content,omitempty"`
}
