package menu

import (
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
)

// RegisterMenuRoutes registers menu management routes on the given group
// (expected to be the scope-dependent /menus group).
func RegisterMenuRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	menuSvc := NewService(ctx)
	menuHandler := NewHandler(menuSvc)
	g.GET("", menuHandler.List)
	g.GET("/", menuHandler.List)
	g.POST("", menuHandler.Create)
	g.POST("/", menuHandler.Create)
	g.PUT("/:id", menuHandler.Update)
	g.DELETE("/:id", menuHandler.Delete)
	g.PUT("/:id/sort", menuHandler.UpdateSort)
}
