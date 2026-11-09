package agent

import (
	"context"
	"fmt"
	"time"

	tcptr "github.com/cuihairu/croupier/core/transport/tcp"
	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	"github.com/cuihairu/croupier/internal/telemetry"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
)

// handleInvoke handles InvokeRequest
func (h *LocalHandler) handleInvoke(ctx context.Context, data []byte) ([]byte, error) {
	req := &sdkv1.InvokeRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("unmarshal InvokeRequest: %w", err)
	}

	functionID := req.GetFunctionId()
	ctx = telemetry.ExtractContext(ctx, req.GetMetadata())
	ctx, span := agentTracer().Start(ctx, "agent.invoke",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("function.id", functionID),
			attribute.String("agent.id", h.agentID),
			attribute.String("service.id", req.GetMetadata()["serviceId"]),
			attribute.String("task.id", req.GetMetadata()["taskId"]),
		),
	)
	defer span.End()

	// Check if this is a provider function call
	h.mu.RLock()
	pm := h.pm
	h.mu.RUnlock()

	if pm != nil && pm.IsPlatformFunction(functionID) {
		resp, err := h.invokePlatform(ctx, functionID, req)
		recordSpanResult(span, err)
		return resp, err
	}

	// Function adapter plugin dispatch (todo.md:35): functions declared
	// by external plugins are served locally by the plugin host, never
	// forwarded to game instances — the plugin IS the mapping to the
	// game interface. Params ride the protocol-v1 JSON payload.
	h.mu.RLock()
	pfm := h.pluginFuncs
	h.mu.RUnlock()
	if pfm != nil && pfm.IsPluginFunction(functionID) {
		respPayload, err := pfm.CallPluginFunction(ctx, functionID, req.GetPayload())
		if err != nil {
			recordSpanResult(span, err)
			return nil, err
		}
		out, err := proto.Marshal(&sdkv1.InvokeResponse{Payload: respPayload})
		recordSpanResult(span, err)
		return out, err
	}

	// R30-09：任务延续（drain 前已 ACK 的 StartTask）不得被 drain 的
	// agentlocal 摘除截断。drain 只清 agentlocal 实例表，不清会话存储；
	// callProvider 会按会话存储选路。因此任务延续跳过 pickInstance
	//（其读 agentlocal，drain 后必然报未注册），直接走会话路由。新业务
	// 请求（无 taskId）仍走 pickInstance 的 agentlocal 快照。
	addr := ""
	isTaskContinuation := req.GetMetadata()["taskId"] != ""
	h.mu.RLock()
	haveSessions := h.providerSessions != nil
	h.mu.RUnlock()
	if isTaskContinuation && haveSessions {
		// 路由交给 callProvider：它按 serviceId/函数匹配会话存储，任务
		// 延续在 draining 会话上放行；无会话命中则回落 legacy 拨号
		//（fallbackAddr 空 → 明确失败，不误报为未注册的 5xx）。
	} else {
		var err error
		addr, err = h.pickInstance(functionID, req.GetMetadata())
		if err != nil {
			recordSpanResult(span, err)
			return nil, err
		}
		span.SetAttributes(attribute.String("provider.addr", addr))
	}

	callCtx, cancel := context.WithTimeout(ctx, h.providerCallDeadline(req.GetMetadata()))
	defer cancel()

	// req 是函数开头 proto.Unmarshal 的产物，字段值必为合法 wire 形态，
	// 再 Marshal 恒成功，error 分支不可达，已删除。
	reqBytes, _ := proto.Marshal(req)

	respBytes, err := h.callProvider(callCtx, functionID, req.GetMetadata(), addr, protocol.MsgInvokeRequest, reqBytes)
	if err != nil {
		if addr != "" {
			err = fmt.Errorf("invoke local provider at %s: %w", addr, err)
		} else {
			err = fmt.Errorf("invoke local provider: %w", err)
		}
		recordSpanResult(span, err)
		return nil, err
	}

	resp := &sdkv1.InvokeResponse{}
	if err := proto.Unmarshal(respBytes, resp); err != nil {
		err = fmt.Errorf("unmarshal InvokeResponse: %w", err)
		recordSpanResult(span, err)
		return nil, err
	}

	out, err := proto.Marshal(resp)
	recordSpanResult(span, err)
	return out, err
}

func (h *LocalHandler) callProvider(ctx context.Context, functionID string, metadata map[string]string, fallbackAddr string, msgID uint32, data []byte) ([]byte, error) {
	h.mu.RLock()
	sessions := h.providerSessions
	h.mu.RUnlock()
	if sessions != nil {
		// 任务延续（drain 前已接受的 StartTask）不得被 drain 截断：其
		// provider 调用仍允许落在 draining 会话上，在途计数照常增减，
		// drain 等待窗内可见。新业务请求（无 taskId）则按 contract 跳过
		// draining 会话、全部候选 drain 时显式拒绝。
		isTaskContinuation := metadata["taskId"] != ""
		serviceID := metadata["serviceId"]
		if serviceID != "" {
			if session, ok := sessions.GetByServiceID(serviceID); ok && session.Conn() != nil {
				if session.Draining() && !isTaskContinuation {
					return nil, providerDrainingError(serviceID)
				}
				return callSession(ctx, session, msgID, data)
			}
		}
		var allDrainingService string
		for _, session := range sessions.List() {
			if session.Conn() == nil {
				continue
			}
			for _, id := range session.FunctionIDs() {
				if id != functionID {
					continue
				}
				if session.Draining() && !isTaskContinuation {
					allDrainingService = session.ServiceID
					continue
				}
				return callSession(ctx, session, msgID, data)
			}
		}
		if allDrainingService != "" {
			return nil, providerDrainingError(allDrainingService)
		}
		// R30-09：任务延续在 drain 后已无会话/agentlocal 可选（会话存储
		// 仍存在但无函数匹配，或 serviceId 未命中）。fallbackAddr 空即
		// 任务延续路径跳过 pickInstance 后未选到地址——此时必须明确失败，
		// 不能拿空地址去拨号。新业务请求若未命中会话，仍有 pickInstance
		// 给的 addr 可回落。
		if isTaskContinuation && fallbackAddr == "" {
			return nil, fmt.Errorf("no provider session for task continuation of function %q", functionID)
		}
	}
	return h.callLocalProvider(ctx, fallbackAddr, msgID, data)
}

// callSession 在选中会话上执行一次 provider 调用，并把调用计入在途计数：
// drain 的 ACK 等待据此知道何时在途归零。调用返回（含出错）即出队。
func callSession(ctx context.Context, session *ProviderSession, msgID uint32, data []byte) ([]byte, error) {
	session.AddInflight(1)
	defer session.AddInflight(-1)
	_, response, err := session.Conn().Call(ctx, msgID, data)
	return response, err
}

// callLocalProvider calls a local provider using TCP transport only
func (h *LocalHandler) callLocalProvider(ctx context.Context, addr string, msgID uint32, data []byte) ([]byte, error) {
	h.mu.RLock()
	tlsCfg := h.tlsCfg
	h.mu.RUnlock()
	// Only TCP transport is supported for LocalHandler.
	// outboundTLS 配置段经 SetTLSConfig 注入；非 nil 时按证书文件拨号 TLS，
	// 零值回退明文（能力矩阵批次 F 死接线修复：此前 tlsCfg 存入即丢，
	// 配置显式开启 outboundTLS 也不产生任何效果）。
	cfg := &tcptr.Config{
		Address:        addr,
		Insecure:       tlsCfg == nil,
		ConnectTimeout: 5 * time.Second,
		RecvTimeout:    30 * time.Second,
		SendTimeout:    30 * time.Second,
	}
	if tlsCfg != nil {
		cfg.CertFile = tlsCfg.CertFile
		cfg.KeyFile = tlsCfg.KeyFile
		cfg.CAFile = tlsCfg.CAFile
		cfg.ServerName = tlsCfg.ServerName
		cfg.InsecureSkipVerify = tlsCfg.InsecureSkipVerify
	}
	client, err := tcptr.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()
	_, respBody, err := client.Call(ctx, msgID, data)
	return respBody, err
}

// invokePlatform handles provider function calls
func (h *LocalHandler) invokePlatform(ctx context.Context, functionID string, req *sdkv1.InvokeRequest) ([]byte, error) {
	request := req.GetPayload()

	h.mu.RLock()
	pm := h.pm
	h.mu.RUnlock()

	if pm == nil {
		return nil, fmt.Errorf("provider manager not configured")
	}

	response, err := pm.Call(ctx, functionID, request)
	if err != nil {
		return nil, fmt.Errorf("provider call failed: %w", err)
	}

	resp := &sdkv1.InvokeResponse{
		Payload: response,
	}
	return proto.Marshal(resp)
}

// pickInstance selects a game server instance for the function.
// Uses double-index routing: function_id -> service_id -> instances (Nacos style)
func (h *LocalHandler) pickInstance(functionID string, metadata map[string]string) (string, error) {
	if h.store == nil {
		return "", fmt.Errorf("instance store not initialized")
	}
	if functionID == "" {
		return "", fmt.Errorf("function ID is required")
	}

	// 使用双层索引获取实例
	snap := h.store.ListByService()
	serviceMap := snap[functionID]
	if len(serviceMap) == 0 {
		return "", fmt.Errorf("function '%s' is not registered", functionID)
	}

	// 按 service_id 路由（一级过滤）
	targetService := metadata["serviceId"]
	var arr []agentlocal.Instance
	if targetService != "" {
		arr = serviceMap[targetService]
		if len(arr) == 0 {
			return "", fmt.Errorf("no instance for service '%s'", targetService)
		}
	} else {
		// 无 service_id，合并所有实例
		for _, instances := range serviceMap {
			arr = append(arr, instances...)
		}
	}

	// 优先选择健康实例（30秒内有心跳）
	// instanceHealthyWindow 是可注入缝隙（默认与生产行为一致），测试中
	// 设为负值可让全部实例落入 stale 降级分支。
	now := time.Now()
	var healthy []agentlocal.Instance
	for _, inst := range arr {
		if now.Sub(inst.LastSeen) < instanceHealthyWindow {
			healthy = append(healthy, inst)
		}
	}

	if len(healthy) > 0 {
		// 使用 fnvIndex 做负载均衡（round-robin风格）
		idx := fnvIndex(functionID, len(healthy))
		return healthy[idx].Addr, nil
	}

	// 降级：返回第一个实例
	h.logger.Warn("function instances are stale, routing to first instance anyway",
		"function_id", functionID, "service_id", targetService, "addr", arr[0].Addr)
	return arr[0].Addr, nil
}
