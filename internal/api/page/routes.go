package page

import (
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
	group.POST("/proposals/rebuild", handler.RebuildProposals)
	group.POST("/:pageKey/regenerate", handler.RegenerateDraft)
	group.POST("/:pageKey/validate", handler.Validate)
	group.POST("/:pageKey/preview", handler.Preview)
	group.POST("/:pageKey/publish", handler.Publish)
	group.POST("/:pageKey/unpublish", handler.Unpublish)
	group.GET("/:pageKey/versions", handler.Versions)
	group.GET("/:pageKey/versions/:versionId", handler.VersionDetail)
	group.POST("/:pageKey/rollback", handler.Rollback)
}
