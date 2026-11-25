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

	// Handshake auth (CR-FIX-11): validated before the session is registered; on
	// rejection return an error and register no session (fail-closed). Token
	// extraction must happen before metadata merging — merging drops
	// reserved keys (including authToken). When require=false the token is
	// ignored entirely.
	h.mu.RLock()
	authRequire := h.providerAuthRequired
	authToken := h.providerAuthToken
	h.mu.RUnlock()
	if err := checkProviderAuth(authRequire, authToken, extractProviderAuthToken(req)); err != nil {
		h.logger.Warn("provider handshake auth rejected", "service_id", req.ServiceId, "error", err)
		return nil, err
	}

	// Verify the SDK's game_id/env against the Agent config (scope spec §14:
	// the provider must report a scope matching the agent — there is no
	// empty-value tolerance, empty is a mismatch; the agent-side scope is
	// guaranteed non-empty by startup validation).
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
		// Extract metadata (modeled on Nacos metadata): the fixed fields are
		// platform reserved keys, and the user-declared multi-KV instance
		// metadata (serverId, etc.) in ProviderConnectRequest.metadata is
		// merged in verbatim — reserved-key conflicts are dropped with a
		// warning, preventing platform semantics from being overwritten.
		// Shares MergeUserProviderMetadata with the TCP provider path, so the
		// semantics are identical.
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
// undrain is expressed by the reason == protocol.ProviderDrainReasonUndrain
// sentinel. The ACK is only returned once drain completes (in-flight reaches
// zero) or the grace period elapses.
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

// DrainProviderSession is the shared agent-side implementation of provider
// drain: both the server-initiated node-level/single-session drain
// (handleProviderDrain) and the SDK-initiated self drain (sidecar
// handleDrain) go through it; the two share the same ProviderSessionStore
// (the internal/app/agent wiring: SetProviderSessionStore and the sidecar
// listener are the same instance).
//
// Semantics (wire contract §drain):
//   - drain: mark the target session draining (callProvider no longer routes
//     new business requests to it), and synchronously remove it from the
//     agentlocal instance table (the legacy address dial fallback no longer
//     routes there), then wait for the target session's in-flight count to
//     reach zero or the grace window to elapse; waiting cancels no in-flight
//     execution.
//   - undrain: only clear the draining flag; the agentlocal instance table is
//     not repopulated (the provider must re-register or refresh its functions
//     via heartbeat — honest boundary).
//
// An empty sessionID applies to all sessions (node level). Returns the number
// of target sessions.
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

	// When providerSessions is not wired (tests / degraded wiring) or no target
	// session is hit, degrade to agentlocal removal (the old semantics) so that
	// handleProviderDrain can still clear the instance table.
	if len(targets) == 0 && h.store != nil {
		h.store.RemoveProvider(sessionID)
	}

	for _, sess := range targets {
		sess.SetDraining(!undrain)
		if undrain {
			continue
		}
		// Remove the agentlocal instance: the legacy address dial from
		// pickInstance/callLocalProvider no longer routes to this session;
		// session-level function routing is backstopped by callProvider's
		// draining skip.
		if h.store != nil {
			h.store.RemoveProvider(sess.SessionID)
		}
	}

	if undrain || len(targets) == 0 {
		return len(targets), nil
	}

	// Wait for in-flight to reach zero or the grace to elapse. In-flight
	// executions are not cancelled; on expiry we return and let tasks still
	// running converge via ARCH-015 D3 (unknown/terminal events), rather than
	// being force-killed by drain.
	// R30-09: beyond session in-flight, we must also wait for tasks that are
	// "ACKed but not yet entered the Provider RPC" (TaskRunner.Count()>0).
	// Within that window the in-flight count is 0, so a drain that only checks
	// inflight would ACK immediately and remove agentlocal, after which
	// pickInstance in executeTask→handleInvoke would report not-registered —
	// truncating the task. Waiting for Count to reach zero (or the grace to
	// elapse) lets accepted tasks finish, and the task continuation path has
	// already bypassed pickInstance (see handleInvoke).
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
