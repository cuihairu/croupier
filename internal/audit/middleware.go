package audit

import (
	"github.com/gin-gonic/gin"
)

// DegradedHeader 是 ARCH-007 D6 的响应头名。当审计存储写入失败且事件
// 走失败开放路径时置 true，供客户端/网关观测审计降级（业务仍成功）。
const DegradedHeader = "X-Audit-Degraded"

// DegradedMiddleware 在每个请求植入审计降级标志，handler 之后的
// LogOrDegrade（失败开放路径）set 该标志，响应阶段据此写
// X-Audit-Degraded: true。挂在 reqinfo.Middleware 之后、业务路由之前。
func DegradedMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(WithDegradedFlag(c.Request.Context()))
		c.Next()
		if WasDegraded(c.Request.Context()) {
			c.Header(DegradedHeader, "true")
		}
	}
}
