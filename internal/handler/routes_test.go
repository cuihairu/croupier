package handler

import (
	"testing"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
)

func TestRegisterHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	ctx := &svc.ServiceContext{}
	RegisterHandlers(r, ctx)
}
