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

	// R30-09: task continuation (a StartTask already ACKed before drain) must
	// not be truncated by drain removing agentlocal. Drain only clears the
	// agentlocal instance table, not the session store; callProvider routes
	// by the session store. So task continuation skips pickInstance (which
	// reads agentlocal and would certainly report not-registered after drain)
	// and goes straight to session routing. New business requests (no taskId)
	// still go through pickInstance's agentlocal snapshot.
	addr := ""
	isTaskContinuation := req.GetMetadata()["taskId"] != ""
	h.mu.RLock()
	haveSessions := h.providerSessions != nil
	h.mu.RUnlock()
	if isTaskContinuation && haveSessions {
		// Routing is delegated to callProvider: it matches the session store
		// by serviceId/function, and lets task continuation through on
		// draining sessions; with no session hit it falls back to the legacy
		// dial (empty fallbackAddr → explicit failure, not misreported as a
		// not-registered 5xx).
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

	// req is the product of the proto.Unmarshal at the top of the function, so its
	// field values are always valid wire form; marshaling it again always
	// succeeds, the error branch was unreachable and has been deleted.
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
		// Task continuation (a StartTask already accepted before drain) must not be
		// truncated by drain: its provider call is still allowed to land on a
		// draining session, with in-flight counting incrementing/decrementing as
		// usual and visible within the drain wait window. New business requests
		// (no taskId) instead skip draining sessions per contract, and are
		// explicitly rejected when all candidates are draining.
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
		// R30-09: task continuation has no session/agentlocal left to choose after
		// drain (the session store still exists but no function matches, or the
		// serviceId missed). An empty fallbackAddr means the task continuation
		// path skipped pickInstance and selected no address — it must then fail
		// explicitly instead of dialing an empty address. New business requests
		// that miss a session still have the addr from pickInstance to fall
		// back to.
		if isTaskContinuation && fallbackAddr == "" {
			return nil, fmt.Errorf("no provider session for task continuation of function %q", functionID)
		}
	}
	return h.callLocalProvider(ctx, fallbackAddr, msgID, data)
}

// callSession performs one provider call on the selected session and counts the
// call in the in-flight tally: the drain ACK wait uses this to know when
// in-flight reaches zero. The call dequeues when it returns (including on
// error).
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
	// The outboundTLS config section is injected via SetTLSConfig; when it is
	// non-nil, dial TLS using the certificate files, and fall back to plaintext
	// for the zero value (capability matrix batch F dead wiring fix: tlsCfg
	// used to be dropped as soon as it was stored, so explicitly enabling
	// outboundTLS had no effect at all).
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

	// Use the double-level index to fetch instances
	snap := h.store.ListByService()
	serviceMap := snap[functionID]
	if len(serviceMap) == 0 {
		return "", fmt.Errorf("function '%s' is not registered", functionID)
	}

	// Route by service_id (first-level filter)
	targetService := metadata["serviceId"]
	var arr []agentlocal.Instance
	if targetService != "" {
		arr = serviceMap[targetService]
		if len(arr) == 0 {
			return "", fmt.Errorf("no instance for service '%s'", targetService)
		}
	} else {
		// No service_id: merge all instances
		for _, instances := range serviceMap {
			arr = append(arr, instances...)
		}
	}

	// Prefer healthy instances (heartbeat within 30 seconds)
	// instanceHealthyWindow is an injectable seam (default matches production
	// behavior); tests set it negative to push every instance into the stale
	// degradation branch.
	now := time.Now()
	var healthy []agentlocal.Instance
	for _, inst := range arr {
		if now.Sub(inst.LastSeen) < instanceHealthyWindow {
			healthy = append(healthy, inst)
		}
	}

	if len(healthy) > 0 {
		// Use fnvIndex for load balancing (round-robin style)
		idx := fnvIndex(functionID, len(healthy))
		return healthy[idx].Addr, nil
	}

	// Degradation: return the first instance
	h.logger.Warn("function instances are stale, routing to first instance anyway",
		"function_id", functionID, "service_id", targetService, "addr", arr[0].Addr)
	return arr[0].Addr, nil
}
