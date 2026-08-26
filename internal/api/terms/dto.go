package terms

// TermDeleteRequest represents the request to delete a term
type TermDeleteRequest struct {
	Domain string `json:"domain"` // resource | operation
	Alias  string `json:"alias"`  // 要删除的别名
}

// TermDeleteResponse represents the response for term delete
type TermDeleteResponse struct {
	Ok bool `json:"ok"`
}

// TermItem represents a single term dictionary item.
// Display 是本地化显示文本，key 必须是 BCP47 locale（"zh-CN"/"en-US"），
// 与 spec.LocalizedText 契约一致。
type TermItem struct {
	Id      int64             `json:"id"`
	Domain  string            `json:"domain"`  // resource | operation
	TermKey string            `json:"termKey"` // 原始术语键
	Alias   string            `json:"alias"`   // 别名
	Display map[string]string `json:"display"` // BCP47 locale -> 显示文本
	Order   int64             `json:"order"`   // 排序
}

// TermUpsertRequest represents the request to create or update a term.
// Display 的 key 统一归一为 BCP47；非法/空 key 会被丢弃。
type TermUpsertRequest struct {
	Domain  string            `json:"domain"`  // resource | operation
	TermKey string            `json:"termKey"` // 原始术语键
	Alias   string            `json:"alias"`   // 别名
	Display map[string]string `json:"display"` // BCP47 locale -> 显示文本
	Order   int64             `json:"order"`   // 排序
}

// TermUpsertResponse represents the response for term upsert
type TermUpsertResponse struct {
	Ok bool `json:"ok"`
}

// TermsListRequest represents the request to list terms
type TermsListRequest struct {
	Domain string `form:"domain" json:"domain"` // resource | operation
}

// TermsListResponse represents the response with terms list
type TermsListResponse struct {
	Items []TermItem `json:"items"`
}
