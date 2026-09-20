package page

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
)

// RegisterDraftRoutes mounts the canonical PageSpec draft API on group.
// Both server entry points use this function so their HTTP contracts cannot
// drift apart.
func RegisterDraftRoutes(group *gin.RouterGroup, svcCtx *svc.ServiceContext) {
	handler := NewHandler(NewService(svcCtx))
	group.GET("", handler.ListDrafts)
	group.GET("/", handler.ListDrafts)
	group.GET("/:pageKey", handler.GetDraft)
	group.PUT("/:pageKey", handler.SaveDraft)
	group.PUT("/:pageKey/menu", handler.SetMenu)
	group.POST("/proposals/rebuild", handler.RebuildProposals)
	group.POST("/:pageKey/regenerate", handler.RegenerateDraft)
	group.POST("/:pageKey/sync-selectors", handler.SyncSelectors)
	group.POST("/:pageKey/validate", handler.Validate)
	group.POST("/:pageKey/preview", handler.Preview)
	group.POST("/:pageKey/publish", handler.Publish)
	group.POST("/:pageKey/unpublish", handler.Unpublish)
	group.POST("/bulk-publish", handler.BulkPublish)
	group.POST("/bulk-unpublish", handler.BulkUnpublish)
	group.POST("/bulk-republish", handler.BulkRepublish)
	group.POST("/bulk-sync-selectors", handler.BulkSyncSelectors)
	group.POST("/seed-demo", handler.SeedDemoData)
	group.GET("/:pageKey/versions", handler.Versions)
	group.GET("/:pageKey/versions/:versionId", handler.VersionDetail)
	group.POST("/:pageKey/rollback", handler.Rollback)

	// 契约漂移后的 stale 自动愈合循环（2026-09 自动化收口）：能自动适配的
	// 页面由系统同步+发布恢复，修不了的留在编辑器告警。多实例并发由草稿
	// 乐观锁与幂等护栏（无变化不落库）兜底。
	go NewService(svcCtx).StartStaleHealLoop(context.Background(), 5*time.Minute)
}
