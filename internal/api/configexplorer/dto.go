package configexplorer

// BindingDTO is the API shape of a config source binding（config 已脱敏）.
type BindingDTO struct {
	ID        uint   `json:"id"`
	GameID    string `json:"gameId"`
	Env       string `json:"env"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Config    string `json:"config"`
	Writable  bool   `json:"writable"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// BindingUpsertRequest creates or updates a binding.
type BindingUpsertRequest struct {
	ID     uint   `json:"id"`
	GameID string `json:"gameId"`
	Env    string `json:"env"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Config string `json:"config"`
}

// EntryDTO mirrors configsource.Entry（保持 lowerCamelCase 契约）.
type EntryDTO struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Dir     bool   `json:"dir"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime,omitempty"`
}

// FileResponse is the file content；二进制（xlsx 等）走 base64.
type FileResponse struct {
	Path     string `json:"path"`
	Format   string `json:"format"`           // 按扩展名推断：json/csv/xlsx/lua/python/...
	Text     string `json:"text,omitempty"`   // 文本内容（文本格式）
	Base64   string `json:"base64,omitempty"` // 二进制内容（xlsx 等）
	Size     int64  `json:"size"`
	Writable bool   `json:"writable"`
}

// WriteRequest is the emergency edit payload.
type WriteRequest struct {
	SourceID uint   `json:"sourceId"`
	Path     string `json:"path"`
	Content  string `json:"content"` // 文本写回（二进制编辑不支持）
	Reason   string `json:"reason"`  // 应急原因（必填，入审计）
}
