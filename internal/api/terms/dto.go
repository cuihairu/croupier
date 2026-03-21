package terms

// TermDeleteRequest represents the request to delete a term
type TermDeleteRequest struct {
	Domain string `json:"domain"` // entity | operation
	Alias  string `json:"alias"`  // 要删除的别名
}

// TermDeleteResponse represents the response for term delete
type TermDeleteResponse struct {
	Ok bool `json:"ok"`
}

// TermItem represents a single term dictionary item
type TermItem struct {
	Id        int64  `json:"id"`
	Domain    string `json:"domain"`     // entity | operation
	TermKey   string `json:"term_key"`   // 原始术语键
	Alias     string `json:"alias"`      // 别名
	DisplayZh string `json:"display_zh"` // 中文显示
	DisplayEn string `json:"display_en"` // 英文显示
	Order     int64  `json:"order"`      // 排序
}

// TermUpsertRequest represents the request to create or update a term
type TermUpsertRequest struct {
	Domain    string `json:"domain"`     // entity | operation
	TermKey   string `json:"term_key"`   // 原始术语键
	Alias     string `json:"alias"`      // 别名
	DisplayZh string `json:"display_zh"` // 中文显示
	DisplayEn string `json:"display_en"` // 英文显示
	Order     int64  `json:"order"`      // 排序
}

// TermUpsertResponse represents the response for term upsert
type TermUpsertResponse struct {
	Ok bool `json:"ok"`
}

// TermsListRequest represents the request to list terms
type TermsListRequest struct {
	Domain string `form:"domain" json:"domain"` // entity | operation
}

// TermsListResponse represents the response with terms list
type TermsListResponse struct {
	Items []TermItem `json:"items"`
}
