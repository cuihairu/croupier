package ops

// GAME-010 slice 4：业务操作凭据的运维面（GAME-009 D4 查询 / D5 取消确认）。
// 作用域走 X-Game-ID/X-Env 头（GameDBMiddleware 注入 ctx），query 携带的
// scope 被静默忽略是既有坑，此处不读；路径参数是唯一入参源（bodyless POST，
// 同进程动作面 #59 口径）。

import (
	"errors"
	"net/http"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/platform/functioninvoke"
	"github.com/cuihairu/croupier/internal/platform/gameops"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
)

type DurableOpsHandler struct {
	svcCtx *svc.ServiceContext
}

func NewDurableOpsHandler(svcCtx *svc.ServiceContext) *DurableOpsHandler {
	return &DurableOpsHandler{svcCtx: svcCtx}
}

// newCancelInvoker 生产装配 DispatcherCancelInvoker（经平台 Dispatcher 派发
// 取消确认函数）；变量化供测试注入 stub——ops 包没有 dispatcher/agent 测试
// 基建，D5 全链语义在 HTTP 层用 stub invoker 验证。
var newCancelInvoker = func(svcCtx *svc.ServiceContext) gameops.CancelInvoker {
	return functioninvoke.NewDispatcherCancelInvoker(svcCtx)
}

// DurableCredentialGet 查询凭据收敛结果（D4 客户端凭 operationId 查询）。
func (h *DurableOpsHandler) DurableCredentialGet(c *gin.Context) {
	operationID := strings.TrimSpace(c.Param("operationId"))
	if operationID == "" {
		response.Error(c, errorx.NewBadRequest("operationId is required"))
		return
	}
	if h.svcCtx == nil || h.svcCtx.DB == nil {
		response.Error(c, serviceUnavailable("business operation credential store unavailable"))
		return
	}
	scope := svc.GameScopeFromContext(c.Request.Context())
	view, err := gameops.QueryCredential(c.Request.Context(), gameops.NewCredentialStore(h.svcCtx.DB), scope.GameID, scope.Env, operationID)
	if err != nil {
		response.Error(c, mapCredentialError(err, operationID))
		return
	}
	response.Success(c, view)
}

// DurableCredentialCancel 取消确认（D5）：平台绝不自行标 canceled，必须拿到
// 游戏侧 cancelHint 函数的权威信封（canceled/cancel_failed）才迁移状态。
func (h *DurableOpsHandler) DurableCredentialCancel(c *gin.Context) {
	operationID := strings.TrimSpace(c.Param("operationId"))
	if operationID == "" {
		response.Error(c, errorx.NewBadRequest("operationId is required"))
		return
	}
	if h.svcCtx == nil || h.svcCtx.DB == nil {
		response.Error(c, serviceUnavailable("business operation credential store unavailable"))
		return
	}
	// Dispatcher 未装配不在前置守卫拦截：先做凭据支持性/状态判定（409），
	// 确认通道不可用在 invoker 层报 ErrCancelChannelUnavailable → 503。
	scope := svc.GameScopeFromContext(c.Request.Context())
	view, err := gameops.ConfirmCancellation(
		c.Request.Context(),
		gameops.NewCredentialStore(h.svcCtx.DB),
		newCancelInvoker(h.svcCtx),
		scope.GameID, scope.Env, operationID,
		perfUsername(c),
	)
	if err != nil {
		response.Error(c, mapCredentialError(err, operationID))
		return
	}
	response.Success(c, view)
}

// mapCredentialError 把 gameops 域错误映射到 HTTP 契约：
//   - 凭据不存在 → 404；
//   - 未声明取消确认函数 → 409 cancel_not_supported（状态性冲突，非参数错误）；
//   - 其余（派发失败/信封非法/outcome 不可采信）原样透传——凭据未迁移，
//     调用方可重试确认。
func mapCredentialError(err error, operationID string) error {
	if errors.Is(err, gameops.ErrCredentialNotFound) {
		return errorx.NewNotFound("business operation credential not found: " + operationID)
	}
	if errors.Is(err, gameops.ErrCancelNotSupported) {
		return errorx.NewConflictWithCode("cancel_not_supported", err.Error(), map[string]any{"operationId": operationID})
	}
	if errors.Is(err, gameops.ErrCancelNotApplicable) {
		return errorx.NewConflictWithCode("cancel_not_applicable", err.Error(), map[string]any{"operationId": operationID})
	}
	if errors.Is(err, gameops.ErrCancelChannelUnavailable) {
		return serviceUnavailable(err.Error())
	}
	return err
}

func serviceUnavailable(message string) *errorx.CodeError {
	return &errorx.CodeError{Code: http.StatusServiceUnavailable, Message: message}
}
