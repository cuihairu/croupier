package handler

import (
	"github.com/cuihairu/croupier/internal/api/audit"
	"github.com/cuihairu/croupier/internal/api/openapi"
	"github.com/cuihairu/croupier/internal/api/ops"
	apiregistry "github.com/cuihairu/croupier/internal/api/registry"
	"github.com/cuihairu/croupier/internal/api/storage"
	functionapi "github.com/cuihairu/croupier/internal/function/api"
	"github.com/cuihairu/croupier/internal/function/registry"
	"github.com/cuihairu/croupier/internal/svc"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// Storage 路由注册
// ============================================================================
func registerStorageRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	storageSvc := storage.NewService(ctx)
	storageHandler := storage.NewHandler(storageSvc)
	g.GET("/signed-url", storageHandler.SignedURL)
	g.GET("/objects", storageHandler.ListObjects)
	g.POST("/objects", storageHandler.UploadObject)
	g.DELETE("/objects", storageHandler.DeleteObject)
	g.POST("/objects/batch-delete", storageHandler.BatchDeleteObjects)
	g.POST("/directories", storageHandler.CreateDirectory)
	g.POST("/directories/rename", storageHandler.RenameDirectory)
}

// ============================================================================
// Registry 路由注册（公开访问，无需认证）
// ============================================================================
func registerRegistryRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	registrySvc := apiregistry.NewService(ctx)
	registryHandler := apiregistry.NewHandler(registrySvc)
	g.GET("/", registryHandler.GetRegistry)
}

// ============================================================================
// Audit 路由注册（公开访问，在 v1 根路径）
// ============================================================================
func registerAuditRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	auditSvc := audit.NewService(ctx)
	auditHandler := audit.NewHandler(auditSvc)
	g.GET("/audit", auditHandler.GetAuditLogs)  // 支持 GET（前端兼容）
	g.POST("/audit", auditHandler.GetAuditLogs) // 支持 POST（原接口）
	g.GET("/audit/export", auditHandler.Export) // SIEM/归档导出（json/csv）
	g.GET("/audit/chain/verify", auditHandler.VerifyChain)
}

// ============================================================================
// OpenAPI 路由注册（公开访问，在 v1 根路径）
// ============================================================================
func registerOpenAPIReadRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	openapiSvc := openapi.NewService(ctx)
	openapiHandler := openapi.NewHandler(openapiSvc)
	g.GET("/functions/:id/openapi", openapiHandler.GetSpec)
	g.POST("/functions/_openapi-batch", openapiHandler.BatchGetSpec)
	g.GET("/openapi/spec", openapiHandler.GetDocument)
}

func registerOpenAPISourceRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	openapiSvc := openapi.NewService(ctx)
	openapiHandler := openapi.NewHandler(openapiSvc)
	g.GET("/sources", openapiHandler.ListSources)
	g.GET("/runtime-sources", openapiHandler.RuntimeSources)
	g.POST("/sources", openapiHandler.CreateSource)
	g.GET("/sources/:sourceId", openapiHandler.GetSource)
	g.PUT("/sources/:sourceId", openapiHandler.UpdateSource)
	g.GET("/sources/:sourceId/diagnostics", openapiHandler.SourceDiagnostics)
	g.POST("/sources/:sourceId/bindings", openapiHandler.CreateBinding)
	g.DELETE("/sources/:sourceId/bindings/:bindingId", openapiHandler.DeleteBinding)
}

// ============================================================================
// 兼容前端的快捷路由
// ============================================================================
func registerRegistryShortcutRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	opsSvc := ops.NewService(ctx)
	opsHandler := ops.NewHandler(opsSvc)
	g.GET("/registry/services", opsHandler.Services)
}

// ============================================================================
// Function Metadata 路由注册（基于新的 FunctionMetadata protobuf）
// ============================================================================
func registerFunctionMetadataRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	// Create a shared registry store for function metadata
	store := registry.NewStore()
	service := functionapi.NewService(store)
	handler := functionapi.NewHandler(service)

	functions := g.Group("/functions")
	{
		functions.GET("", handler.ListFunctions)
		functions.POST("", handler.RegisterFunction)
		functions.GET("/:id", handler.GetFunction)
		functions.PUT("/:id", handler.UpdateFunction)
		functions.DELETE("/:id", handler.DeleteFunction)
		functions.GET("/resources", handler.GetResources)
		functions.GET("/tags", handler.GetTags)
	}
}
