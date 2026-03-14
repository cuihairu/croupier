// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package feedback

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"

	"github.com/cuihairu/croupier/services/server/internal/logic/feedback"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/gin-gonic/gin"
)

// 获取反馈列表
func FeedbackListHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.FeedbackListRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := feedback.NewFeedbackListLogic(c.Request.Context(), svcCtx)
		resp, err := l.FeedbackList(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
