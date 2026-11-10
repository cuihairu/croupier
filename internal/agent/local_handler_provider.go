package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/cuihairu/croupier/internal/function/registrationguard"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// handleProviderConnect handles ProviderConnectRequest from SDK Providers.
// This allows SDK Providers to register their functions with the Agent.
func (h *LocalHandler) handleProviderConnect(ctx context.Context, data []byte) ([]byte, error) {
	req := &sdkv1.ProviderConnectRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal ProviderConnectRequest: %w", err)
	}

	if req.ServiceId == "" {
		return nil, fmt.Errorf("service_id is required")
	}

	// 握手鉴权（CR-FIX-11）：在会话注册之前校验，拒绝即返回错误不注册
	// 会话（fail-closed）。token 提取必须在元数据合并之前——合并会丢弃
	// 保留键（含 authToken）。require=false 时 token 完全忽略。
	h.mu.RLock()
	authRequire := h.providerAuthRequired
	authToken := h.providerAuthToken
	h.mu.RUnlock()
	if err := checkProviderAuth(authRequire, authToken, extractProviderAuthToken(req)); err != nil {
		h.logger.Warn("provider handshake auth rejected", "service_id", req.ServiceId, "error", err)
		return nil, err
	}

	// 校验 SDK 的 game_id/env 是否与 Agent 配置一致（作用域规范 §14：
	// provider 必须上报 scope 且与 agent 一致——无空值兼容，空值即
	// mismatch；agent 侧 scope 由启动校验保证非空）。
	h.mu.RLock()
	expectedGameID := h.expectedGameID
	expectedEnv := h.expectedEnv
	h.mu.RUnlock()
	var warnings []string
	if req.GameId != expectedGameID {
		msg := fmt.Sprintf("game_id mismatch: provider=%q agent=%q", req.GameId, expectedGameID)
		warnings = append(warnings, msg)
		h.logger.Warn("provider game_id mismatch",
			"service_id", req.ServiceId,
			"provider_game_id", req.GameId,
			"agent_game_id", expectedGameID,
		)
	}
	if req.Env != expectedEnv {
		msg := fmt.Sprintf("env mismatch: provider=%q agent=%q", req.Env, expectedEnv)
		warnings = append(warnings, msg)
		h.logger.Warn("provider env mismatch",
			"service_id", req.ServiceId,
			"provider_env", req.Env,
			"agent_env", expectedEnv,
		)
	}

	sessionID := fmt.Sprintf("ps-%d", time.Now().UnixNano())

	// Use sessionID as providerID for tracking
	// Register all functions from the provider in a single call
	if h.store != nil && len(req.Functions) > 0 {
		// Convert proto functions to ProviderFunctionDescriptor
		funcs := make([]*sdkv1.ProviderFunctionDescriptor, 0, len(req.Functions))
		for _, fn := range req.Functions {
			if violation, ok := providerDescriptorPresentationViolation(fn); ok {
				warning := fmt.Sprintf("function_id=%q registers forbidden presentation field %q at %s and is skipped", fn.GetId(), violation.Field, violation.Location)
				warnings = append(warnings, warning)
				h.logger.Warn("provider function registration rejected", "service_id", req.ServiceId, "function_id", fn.GetId(), "field", violation.Field, "location", violation.Location)
				continue
			}
			funcs = append(funcs, &sdkv1.ProviderFunctionDescriptor{
				Id:                fn.GetId(),
				Version:           fn.GetVersion(),
				Tags:              fn.GetTags(),
				Summary:           fn.GetSummary(),
				Description:       fn.GetDescription(),
				OperationId:       fn.GetOperationId(),
				Deprecated:        fn.GetDeprecated(),
				InputSchema:       fn.GetInputSchema(),
				OutputSchema:      fn.GetOutputSchema(),
				Resource:          fn.GetResource(),
				Operation:         fn.GetOperation(),
				Capability:        fn.GetCapability(),
				Execution:         fn.GetExecution(),
				ApprovalRequired:  fn.GetApprovalRequired(),
				ApprovalPolicyKey: fn.GetApprovalPolicyKey(),
				Risk:              fn.GetRisk(),
				Enabled:           fn.GetEnabled(),
				Permission:        fn.GetPermission(),
			})
		}
		// 提取元数据（参考 Nacos metadata）：固定字段为平台保留键，用户在
		// ProviderConnectRequest.metadata 里声明的多 KV 实例元数据（serverId
		// 等）原样并入——保留键冲突丢弃并告警，防止覆盖平台语义。
		// 与 TCP provider 路径共用 MergeUserProviderMetadata，语义一致。
		metadata, metaWarnings := MergeUserProviderMetadata(map[string]string{
			"sdkLanguage":      req.SdkLanguage,
			"sdkVersion":       req.SdkVersion,
			"sdkName":          req.SdkName,
			"protocol_version": req.ProtocolVersion,
			"gameId":           req.GameId,
			"env":              req.Env,
		}, req.GetMetadata())
		for _, warning := range metaWarnings {
			h.logger.Warn("provider metadata reserved key dropped", "service_id", req.ServiceId, "warning", warning)
		}
		warnings = append(warnings, metaWarnings...)
		// Use empty addr here; the TCP onConnect path sets the real address.
		h.store.Register(sessionID, req.ServiceId, "", req.Version, funcs, metadata)
	}

	h.logger.Info("Provider connected via TCP session",
		"service_id", req.ServiceId,
		"version", req.Version,
		"session_id", sessionID,
		"sdk_language", req.SdkLanguage,
		"sdk_version", req.SdkVersion,
		"sdk_name", req.SdkName,
		"game_id", req.GameId,
		"env", req.Env,
		"functions", len(req.Functions),
		"warnings", len(warnings),
	)

	resp := &sdkv1.ProviderConnectResponse{
		SessionId: sessionID,
		Warnings:  warnings,
	}
	return proto.Marshal(resp)
}

func providerDescriptorPresentationViolation(fn *sdkv1.ProviderFunctionDescriptor) (registrationguard.PresentationViolation, bool) {
	if fn == nil {
		return registrationguard.PresentationViolation{}, false
	}
	return registrationguard.FindPresentationViolation(nil, fn.GetInputSchema(), fn.GetOutputSchema())
}

// handleProviderHeartbeat handles ProviderHeartbeatRequest from SDK Providers.
func (h *LocalHandler) handleProviderHeartbeat(ctx context.Context, data []byte) ([]byte, error) {
	req := &sdkv1.ProviderHeartbeatRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal ProviderHeartbeatRequest: %w", err)
	}

	if req.SessionId == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	// Update LastSeen for this provider's instances without touching the
	// registered functions. Must NOT use Register(nil) here — Register has
	// replace semantics, so a nil func list silently clears all of the
	// provider's functions (this was the demo-site "nothing works" root cause).
	if h.store != nil {
		h.store.Heartbeat(req.SessionId)
	}

	resp := &sdkv1.ProviderHeartbeatResponse{}
	return proto.Marshal(resp)
}

// handleProviderDrain handles ProviderDrainRequest (server-initiated drain of
// one session or the whole node). Empty session_id drains all sessions.
// undrain 由 reason == protocol.ProviderDrainReasonUndrain 哨兵表达。ACK 仅
// 在 drain 完成（在途归零）或宽限到点后返回。
func (h *LocalHandler) handleProviderDrain(ctx context.Context, data []byte) ([]byte, error) {
	req := &sdkv1.ProviderDrainRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal ProviderDrainRequest: %w", err)
	}

	undrain := req.Reason == protocol.ProviderDrainReasonUndrain
	grace := defaultDrainGrace
	if req.RetryAfterMs > 0 {
		grace = time.Duration(req.RetryAfterMs) * time.Millisecond
	}

	drained, err := h.DrainProviderSession(ctx, req.SessionId, undrain, grace)
	if err != nil {
		return nil, fmt.Errorf("drain provider sessions: %w", err)
	}
	h.logger.Info("Provider drain processed",
		"session_id", req.SessionId, "undrain", undrain, "drained", drained)

	resp := &sdkv1.ProviderDrainResponse{}
	return proto.Marshal(resp)
}

// DrainProviderSession 是 agent 侧 provider drain 的共享实现：server 发起的
// 节点级/单会话 drain（handleProviderDrain）与 SDK 发起的自 drain（sidecar
// handleDrain）都经此；两者共享同一 ProviderSessionStore（internal/app/agent
// 的装配：SetProviderSessionStore 与 sidecar listener 同实例）。
//
// 语义（wire contract §drain）：
//   - drain：目标会话标记 draining（callProvider 不再给新业务请求选路），
//     并同步从 agentlocal 实例表摘除（legacy 地址拨号 fallback 不再路由），
//     随后等待目标会话在途归零或宽限窗到点；等待不取消任何在途执行。
//   - undrain：仅清除 draining 标记；agentlocal 实例表不回填（provider 需
//     重新注册或心跳刷新函数——如实边界）。
//
// sessionID 为空表示作用于全部会话（节点级）。返回目标会话数。
func (h *LocalHandler) DrainProviderSession(ctx context.Context, sessionID string, undrain bool, grace time.Duration) (int, error) {
	if grace <= 0 {
		grace = defaultDrainGrace
	}

	var targets []*ProviderSession
	if h.providerSessions != nil {
		if sessionID != "" {
			if sess, ok := h.providerSessions.GetBySessionID(sessionID); ok {
				targets = append(targets, sess)
			}
		} else {
			targets = h.providerSessions.List()
		}
	}

	// providerSessions 未装配（测试/退化装配）或目标会话未命中时，退化到
	// agentlocal 摘除（旧语义），保证 handleProviderDrain 仍能清掉实例表。
	if len(targets) == 0 && h.store != nil {
		h.store.RemoveProvider(sessionID)
	}

	for _, sess := range targets {
		sess.SetDraining(!undrain)
		if undrain {
			continue
		}
		// 摘除 agentlocal 实例：pickInstance/callLocalProvider 的 legacy
		// 地址拨号不再路由到该会话；会话级函数路由由 callProvider 的
		// draining 跳过兜底。
		if h.store != nil {
			h.store.RemoveProvider(sess.SessionID)
		}
	}

	if undrain || len(targets) == 0 {
		return len(targets), nil
	}

	// 等待在途归零或宽限到点。不取消在途执行；超窗返回，让仍在跑的任务
	// 经 ARCH-015 D3 收敛（unknown/终态事件），不因 drain 强杀。
	// R30-09：除会话在途外，还须等待「已 ACK 但尚未进入 Provider RPC」的
	// 任务（TaskRunner.Count()>0）。该窗口内在途计数为 0，drain 若只查
	// inflight 会立即 ACK 并摘 agentlocal，随后 executeTask→handleInvoke
	// 的 pickInstance 报未注册——任务被截断。等待 Count 归零（或宽限到点）
	// 让已接受任务完成，且任务延续路径已绕过 pickInstance（见 handleInvoke）。
	deadline := time.NewTimer(grace)
	defer deadline.Stop()
	ticker := time.NewTicker(drainPollInterval)
	defer ticker.Stop()
	for {
		allZero := true
		if h.tasks != nil && h.tasks.Count() > 0 {
			allZero = false
		}
		for _, sess := range targets {
			if sess.Inflight() > 0 {
				allZero = false
				break
			}
		}
		if allZero {
			return len(targets), nil
		}
		select {
		case <-ctx.Done():
			return len(targets), ctx.Err()
		case <-deadline.C:
			return len(targets), nil
		case <-ticker.C:
		}
	}
}
