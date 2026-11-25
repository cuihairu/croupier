package terms

// TermDeleteRequest represents the request to delete a term
type TermDeleteRequest struct {
	Domain string `json:"domain"` // resource | operation
	Alias  string `json:"alias"`  // alias to delete
}

// TermDeleteResponse represents the response for term delete
type TermDeleteResponse struct {
	Ok bool `json:"ok"`
}

// TermItem represents a single term dictionary item.
// Display is the localized display text; keys must be BCP47 locales
// ("zh-CN"/"en-US"), consistent with the spec.LocalizedText contract.
type TermItem struct {
	Id      int64             `json:"id"`
	Domain  string            `json:"domain"`  // resource | operation
	TermKey string            `json:"termKey"` // original term key
	Alias   string            `json:"alias"`   // alias
	Display map[string]string `json:"display"` // BCP47 locale -> display text
	Order   int64             `json:"order"`   // sort order
}

// TermUpsertRequest represents the request to create or update a term.
// Display keys are normalized to BCP47; invalid/empty keys are dropped.
type TermUpsertRequest struct {
	Domain  string            `json:"domain"`  // resource | operation
	TermKey string            `json:"termKey"` // original term key
	Alias   string            `json:"alias"`   // alias
	Display map[string]string `json:"display"` // BCP47 locale -> display text
	Order   int64             `json:"order"`   // sort order
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
