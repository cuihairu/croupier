package page

import "github.com/cuihairu/croupier/internal/dashboard/spec"

type PageDraftListRequest struct {
	ResourceKey string `form:"resourceKey"`
	Status      string `form:"status"`
}

type PageDraftListResponse struct {
	Items []spec.PageSpecDraftSummary `json:"items"`
}

type PageDraftRequest struct {
	PageKey string `uri:"pageKey" binding:"required"`
}

type PageDraftResponse struct {
	spec.PageSpec
	GameID           string                            `json:"gameId,omitempty"`
	Env              string                            `json:"env,omitempty"`
	Status           string                            `json:"status"`
	DraftRevision    int                               `json:"draftRevision"`
	PublishedVersion int                               `json:"publishedVersion,omitempty"`
	MenuID           *int64                            `json:"menuId,omitempty"`
	Diagnostics      []spec.Diagnostic                 `json:"diagnostics,omitempty"`
	BindingFreshness []spec.BindingFreshnessDiagnostic `json:"bindingFreshness,omitempty"`
	UpdatedAt        string                            `json:"updatedAt"`
	UpdatedBy        string                            `json:"updatedBy,omitempty"`
}

// PageMenuUpdateRequest 是 PUT /api/v1/pages/:pageKey/menu 的载荷：
// menuId 为 null 或 0 表示解除挂载；非 0 必须是同 scope 已存在的菜单。
type PageMenuUpdateRequest struct {
	PageKey string `uri:"pageKey" binding:"required"`
	MenuID  *int64 `json:"menuId"`
}

// PageMenuResponse 回显挂载结果。
type PageMenuResponse struct {
	PageKey string `json:"pageKey"`
	MenuID  *int64 `json:"menuId"`
}

type PageSaveRequest struct {
	PageKey       string                     `uri:"pageKey" binding:"required"`
	DraftRevision *int                       `json:"draftRevision"`
	Type          spec.PageType              `json:"type"`
	ResourceKey   string                     `json:"resourceKey,omitempty"`
	Title         map[string]string          `json:"title"`
	Description   map[string]string          `json:"description,omitempty"`
	Category      spec.PageCategorySpec      `json:"category"`
	Order         int                        `json:"order,omitempty"`
	Icon          string                     `json:"icon,omitempty"`
	Navigation    *spec.NavigationSpec       `json:"navigation,omitempty"`
	Resource      *spec.ResourcePageSpec     `json:"resource,omitempty"`
	Operation     *spec.OperationPageSpec    `json:"operation,omitempty"`
	Task          *spec.TaskPageSpec         `json:"task,omitempty"`
	Report        *spec.ReportPageSpec       `json:"report,omitempty"`
	Bindings      []spec.PageFunctionBinding `json:"bindings"`
}

type PageSaveResponse struct {
	PageKey       string `json:"pageKey"`
	DraftRevision int    `json:"draftRevision"`
}

type PageRegenerateRequest struct {
	PageKey       string `uri:"pageKey" binding:"required"`
	DraftRevision *int   `json:"draftRevision"`
}

type PageRegenerateResponse struct {
	PageKey       string                    `json:"pageKey"`
	DraftRevision int                       `json:"draftRevision"`
	Page          spec.PageSpec             `json:"page"`
	Diagnostics   []spec.Diagnostic         `json:"diagnostics,omitempty"`
	Quality       spec.GeneratedPageQuality `json:"quality"`
}

// PageSyncSelectorsRequest 一键同步 stale selector：dryRun=true 只出报告
// 不落库；bindingIds 非空时只同步指定 binding（默认全量）。draftRevision
// 是乐观锁——草稿被并发修改时 409，防止冲掉他人定制。
type PageSyncSelectorsRequest struct {
	PageKey       string   `uri:"pageKey" binding:"required"`
	DraftRevision *int     `json:"draftRevision"`
	DryRun        bool     `json:"dryRun"`
	BindingIDs    []string `json:"bindingIds,omitempty"`
}

type PageSyncSelectorsResponse struct {
	PageKey        string                           `json:"pageKey"`
	DryRun         bool                             `json:"dryRun"`
	Applied        bool                             `json:"applied"`
	DraftRevision  int                              `json:"draftRevision"`
	SyncedBindings []spec.BindingSelectorSyncReport `json:"syncedBindings"`
	// RemainingDiagnostics 是同步草稿的发布级校验结果。同步不保证可
	// 发布（manual_required 项仍需人工处理），错误留在这里由调用方
	// 呈现，不阻断保存。
	RemainingDiagnostics []spec.Diagnostic `json:"remainingDiagnostics,omitempty"`
	// 自动化收口（2026-09）：同步后若无 manual 遗留且页面已发布，
	// 自动接续发布刷新契约快照（无需用户二次操作）。无发布权限或
	// 发布失败时 AutoPublishError 说明原因，草稿保持已同步状态。
	AutoPublished    bool   `json:"autoPublished"`
	AutoPublishError string `json:"autoPublishError,omitempty"`
}

// PageProposalsRebuildResponse reports the scope covered by a bulk proposal
// rebuild. Per-page results are observable through the proposal list API.
type PageProposalsRebuildResponse struct {
	GameID string `json:"gameId"`
	Env    string `json:"env"`
}

type PageValidateRequest struct {
	PageKey string `uri:"pageKey" binding:"required"`
}

type PageValidateResponse struct {
	Valid       bool              `json:"valid"`
	Diagnostics []spec.Diagnostic `json:"diagnostics"`
}

type PagePreviewRequest struct {
	PageKey string `uri:"pageKey" binding:"required"`
}

type PagePreviewResponse struct {
	Page spec.PageSpec `json:"page"`
}

type PagePublishRequest struct {
	PageKey       string `uri:"pageKey" binding:"required"`
	DraftRevision *int   `json:"draftRevision"`
}

type PagePublishResponse struct {
	PageKey          string `json:"pageKey"`
	Published        bool   `json:"published"`
	PublishedVersion int    `json:"publishedVersion"`
}

type PageUnpublishRequest struct {
	PageKey string `uri:"pageKey" binding:"required"`
}

type PageUnpublishResponse struct {
	PageKey   string `json:"pageKey"`
	Published bool   `json:"published"`
}

type PageVersionsRequest struct {
	PageKey string `uri:"pageKey" binding:"required"`
	// Limit caps the page size (default 5, max 100); 0 uses the default.
	Limit int `form:"limit"`
	// Offset skips older versions; newest-first ordering is preserved.
	Offset int `form:"offset"`
}

type PageVersionsResponse struct {
	CurrentDraftRevision    int                    `json:"currentDraftRevision"`
	CurrentPublishedVersion int                    `json:"currentPublishedVersion,omitempty"`
	Total                   int64                  `json:"total"`
	Items                   []spec.PageVersionItem `json:"items"`
}

type PageVersionDetailRequest struct {
	PageKey   string `uri:"pageKey" binding:"required"`
	VersionID string `uri:"versionId" binding:"required"`
}

type PageVersionDetailResponse struct {
	Version   int           `json:"version"`
	Status    string        `json:"status"`
	Message   string        `json:"message,omitempty"`
	CreatedAt string        `json:"createdAt"`
	CreatedBy string        `json:"createdBy,omitempty"`
	Page      spec.PageSpec `json:"page"`
}

type PageRollbackRequest struct {
	PageKey               string `uri:"pageKey" binding:"required"`
	VersionID             string `json:"versionId"`
	ExpectedDraftRevision *int   `json:"expectedDraftRevision"`
}

type PageRollbackResponse struct {
	PageKey       string `json:"pageKey"`
	DraftRevision int    `json:"draftRevision"`
}

// PageBulkRequest 一键发布/一键下架请求（预留过滤参数）。
type PageBulkRequest struct{}

// PageBulkRepublishRequest 一键重新发布请求：pageKeys 为空时处理 scope 内
// 全部契约漂移的已发布页面（与契约变更队列同源评估）。
type PageBulkRepublishRequest struct {
	PageKeys []string `json:"pageKeys,omitempty"`
}

// PageBulkResult 一键发布/下架/重新发布结果。
type PageBulkResult struct {
	Total       int                 `json:"total"`
	Published   []string            `json:"published,omitempty"`
	Unpublished []string            `json:"unpublished,omitempty"`
	Skipped     []string            `json:"skipped,omitempty"`
	Failed      []map[string]string `json:"failed,omitempty"`
}

// PageBulkSyncSelectorsRequest 批量 selector 同步请求：pageKeys 为空时
// 处理 scope 内全部契约变更页面（与契约变更队列同源评估）。
type PageBulkSyncSelectorsRequest struct {
	PageKeys []string `json:"pageKeys,omitempty"`
}

// PageBulkSyncSelectorsSkipped 批量同步中被跳过的页面：Manual 非空
// （governance/version 等不可由 selector 同步修复的漂移）时整页跳过，
// 诊断透传给人工处理。
type PageBulkSyncSelectorsSkipped struct {
	PageKey string            `json:"pageKey"`
	Reason  string            `json:"reason"`
	Manual  []spec.Diagnostic `json:"manual,omitempty"`
}

// PageBulkSyncSelectorsResult 批量 selector 同步结果。同步只写 draft，
// 上线仍需 bulk-republish（不自动发布）。
type PageBulkSyncSelectorsResult struct {
	Total   int                            `json:"total"`
	Synced  []string                       `json:"synced,omitempty"`
	Skipped []PageBulkSyncSelectorsSkipped `json:"skipped,omitempty"`
	Failed  []map[string]string            `json:"failed,omitempty"`
}
