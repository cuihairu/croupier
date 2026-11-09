package dispatch

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/telemetry"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
)

// BroadcastAgentResult captures the outcome of a single agent invocation
// within a broadcast. Either Response or Err is set.
type BroadcastAgentResult struct {
	AgentID  string
	Response *sdkv1.InvokeResponse
	Err      error
}

// BroadcastInvocation aggregates per-agent outcomes from InvokeBroadcast.
// Successes and Failures together always sum to Total.
type BroadcastInvocation struct {
	Total     int
	Successes []*BroadcastAgentResult
	Failures  []*BroadcastAgentResult
}

// InvokeBroadcast delivers the same request to every live agent that owns the
// function and returns the per-agent outcomes. Individual agent failures are
// captured in the result instead of aborting the whole broadcast, so callers
// can act on partial results.
func (d *Dispatcher) InvokeBroadcast(ctx context.Context, req *sdkv1.InvokeRequest) (*BroadcastInvocation, error) {
	if req == nil || req.GetFunctionId() == "" {
		return nil, fmt.Errorf("function id is required")
	}
	ctx = telemetry.ExtractContext(ctx, req.Metadata)
	ctx, span := dispatchTracer().Start(ctx, "function.dispatch.broadcast",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String("function.id", req.GetFunctionId())),
	)
	defer span.End()

	gameID, env, scoped, err := routingScopeFromMetadata(req.Metadata)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	agents := d.listAgentsForFunctionInScope(req.GetFunctionId(), gameID, env, scoped)
	// broadcast 候选集取 local ∪ remote：语义是投给所有活跃 agent，「本地
	// 空才查远端」会静默漏投对端实例持有的一半（与 pick 的兜底语义不同）。
	agents = d.withRemoteBroadcastCandidates(ctx, agents, req.GetFunctionId(), gameID, env, scoped)
	if len(agents) == 0 {
		err := d.noLiveAgentError(req.GetFunctionId(), gameID, env, scoped)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetAttributes(attribute.Int("agent.count", len(agents)))

	result := &BroadcastInvocation{Total: len(agents)}

	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)

	for _, agent := range agents {
		wg.Add(1)
		go func(agent *reg.AgentSession) {
			defer wg.Done()

			// Per-agent clone so concurrent metadata writes don't race.
			localReq := proto.Clone(req).(*sdkv1.InvokeRequest)
			if localReq.Metadata == nil {
				localReq.Metadata = map[string]string{}
			}
			localReq.Metadata["agentId"] = agent.AgentID
			localReq.Metadata = telemetry.InjectContext(ctx, localReq.Metadata)

			if d.healthTracker != nil {
				d.healthTracker.IncrementConnections(agent.AgentID)
				defer d.healthTracker.DecrementConnections(agent.AgentID)
			}

			out := &BroadcastAgentResult{AgentID: agent.AgentID}

			reqBytes, err := proto.Marshal(localReq)
			if err != nil {
				out.Err = fmt.Errorf("marshal request: %w", err)
			} else {
				// 远端候选（session 在对端实例）经 callAgentRouted 转发执行；
				// 转发失败与其他 agent 失败同语义落 Failures（半失败不整单报错）。
				respBytes, callErr := d.callAgentRouted(ctx, agent.AgentID, protocol.MsgInvokeRequest, localReq, reqBytes)
				if callErr != nil {
					out.Err = callErr
				} else {
					resp := &sdkv1.InvokeResponse{}
					if err := proto.Unmarshal(respBytes, resp); err != nil {
						out.Err = fmt.Errorf("unmarshal response: %w", err)
					} else {
						out.Response = resp
					}
				}
			}

			mu.Lock()
			if out.Err != nil {
				result.Failures = append(result.Failures, out)
			} else {
				result.Successes = append(result.Successes, out)
			}
			mu.Unlock()

			if d.healthTracker != nil {
				if out.Err != nil {
					d.healthTracker.RecordFailure(agent.AgentID)
				} else {
					d.healthTracker.RecordSuccess(agent.AgentID)
				}
			}
		}(agent)
	}

	wg.Wait()
	span.SetAttributes(attribute.Int("function.broadcast.success", len(result.Successes)), attribute.Int("function.broadcast.failure", len(result.Failures)))
	if len(result.Failures) > 0 {
		span.SetStatus(codes.Error, "broadcast had failed agent calls")
	} else {
		span.SetStatus(codes.Ok, "")
	}
	return result, nil
}

// listAgentsForFunction returns every live agent that currently exposes the
// given function. It is kept for scope-neutral diagnostics and legacy callers.
func (d *Dispatcher) listAgentsForFunction(functionID string) []*reg.AgentSession {
	return d.listAgentsForFunctionInScope(functionID, "", "", false)
}

// listAgentsForFunctionInScope returns live agents that expose functionID in
// the requested game/environment. Scoped invocations must never be delivered
// to an Agent registered by another game or environment.
func (d *Dispatcher) listAgentsForFunctionInScope(functionID, gameID, env string, scoped bool) []*reg.AgentSession {
	now := time.Now()

	d.store.Mu().RLock()
	defer d.store.Mu().RUnlock()

	var out []*reg.AgentSession
	for _, agent := range d.store.AgentsUnsafe() {
		if !agentCanInvoke(agent, functionID, now, gameID, env, scoped) {
			continue
		}
		out = append(out, agent)
	}
	return out
}

// withRemoteBroadcastCandidates 把远端候选并入 broadcast 候选集（按
// AgentID 去重、本地优先）。远端目录不可用/为空时返回原列表——broadcast
// 的远端并集是尽力而为，不因归属表故障放大成整单失败。单实例
// （source 未装配）恒返回原列表。
func (d *Dispatcher) withRemoteBroadcastCandidates(ctx context.Context, local []*reg.AgentSession, functionID, gameID, env string, scoped bool) []*reg.AgentSession {
	remote := d.expandRemoteCandidates(ctx, functionID, gameID, env, scoped)
	if len(remote) == 0 {
		return local
	}
	seen := make(map[string]bool, len(local)+len(remote))
	for _, a := range local {
		if a != nil {
			seen[a.AgentID] = true
		}
	}
	out := local
	for _, a := range remote {
		if a == nil || seen[a.AgentID] {
			continue
		}
		seen[a.AgentID] = true
		out = append(out, a)
	}
	return out
}

func agentHasService(agent *reg.AgentSession, providerID, functionID string) bool {
	if agent == nil || providerID == "" {
		return false
	}
	for _, p := range agent.Providers {
		if p.ProviderID != providerID {
			continue
		}
		if functionID == "" {
			return true
		}
		for _, fid := range p.FunctionIDs {
			if fid == functionID {
				return true
			}
		}
		return false
	}
	return false
}

// expandRemoteCandidates 从归属表目录拉对端持有的候选 agent 并按本地
// 同款 agentCanInvoke 过滤（函数注册/enabled/scope/过期）。远端查询带
// 预算，出错或超时返回空（调用方回落 noLiveAgentError 语义，不放大
// 故障）。单实例（source 未装配）恒返回空。
func (d *Dispatcher) expandRemoteCandidates(ctx context.Context, functionID, gameID, env string, scoped bool) []*reg.AgentSession {
	source := d.remoteAgentSource
	if source == nil {
		return nil
	}
	qCtx, cancel := context.WithTimeout(ctx, remoteSourceBudget)
	defer cancel()
	sessions, err := source.RemoteAgentSessions(qCtx, gameID, env, scoped)
	if err != nil {
		log.Printf("[dispatch] remote agent source unavailable (game=%s env=%s): %v", gameID, env, err)
		return nil
	}
	now := time.Now()
	out := make([]*reg.AgentSession, 0, len(sessions))
	for _, agent := range sessions {
		if agentCanInvoke(agent, functionID, now, gameID, env, scoped) {
			out = append(out, agent)
		}
	}
	return out
}
