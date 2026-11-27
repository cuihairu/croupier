package functioninvoke

// GAME-009 批准协议 D5 取消确认的生产调用面：把 gameops.ConfirmCancellation
// 需要的游戏侧确认函数派发接到平台 Dispatcher（与 D4 收敛循环同一传输面）。
// 本层只负责把游戏响应原文带回；凭据状态迁移全部由 ConfirmCancellation
// 承担——派发失败不落任何凭据状态（cancel_failed 只能由游戏信封声明，
// R2：平台不发明取消结果）。

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	"github.com/cuihairu/croupier/internal/platform/gameops"
	"github.com/cuihairu/croupier/internal/svc"
)

// DispatcherCancelInvoker 经平台 Dispatcher 派发取消确认函数。
type DispatcherCancelInvoker struct {
	dispatcher *dispatch.Dispatcher
}

// NewDispatcherCancelInvoker 装配生产 invoker；dispatcher 未装配时
// InvokeCancel 返回 ErrCancelChannelUnavailable（确认通道不可用 ≠ 取消成功）。
func NewDispatcherCancelInvoker(svcCtx *svc.ServiceContext) *DispatcherCancelInvoker {
	if svcCtx == nil {
		return &DispatcherCancelInvoker{}
	}
	return &DispatcherCancelInvoker{dispatcher: svcCtx.Dispatcher}
}

func (i *DispatcherCancelInvoker) InvokeCancel(ctx context.Context, functionID string, payload []byte, gameID, env, actor string) (json.RawMessage, error) {
	if i == nil || i.dispatcher == nil {
		return nil, gameops.ErrCancelChannelUnavailable
	}
	metadata := map[string]string{
		"async":         "false",
		"gameId":        gameID,
		"env":           env,
		"actor":         actor,
		"cancelConfirm": "true",
	}
	resp, err := i.dispatcher.InvokeRequest(ctx, utils.BuildInvokeRequest(strings.TrimSpace(functionID), payload, metadata))
	if err != nil {
		return nil, err
	}
	if resp == nil || len(resp.GetPayload()) == 0 {
		return nil, errors.New("cancel confirmation returned empty response")
	}
	return json.RawMessage(resp.GetPayload()), nil
}
