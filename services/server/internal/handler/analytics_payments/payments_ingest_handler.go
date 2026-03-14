// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package analytics_payments

import (
	"github.com/cuihairu/croupier/services/server/internal/common/response"

	"github.com/cuihairu/croupier/services/server/internal/logic/analytics_payments"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

"github.com/gin-gonic/gin"
)

// 采集支付数据
func PaymentsIngestHandler(svcCtx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.PaymentsIngestRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, err)
			return
		}

		l := analytics_payments.NewPaymentsIngestLogic(c.Request.Context(), svcCtx)
		resp, err := l.PaymentsIngest(&req)
		if err != nil {
			response.Error(c, err)
		} else {
			response.Success(c, resp)
		}
	}
}
