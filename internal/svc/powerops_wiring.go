package svc

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	dispatch "github.com/cuihairu/croupier/internal/platform/dispatch"
	"github.com/cuihairu/croupier/internal/powerops"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/getkin/kin-openapi/openapi3"
	"google.golang.org/protobuf/proto"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// poweropsApprovalGate 适配 approvals.Store 到 powerops.ApprovalGate（provisionApprovalGate
// 同构）：审批记录 FunctionID=ops.power.<action>，Mode=powerops，Metadata 携带
// 请求 id/动作/目标数/风险词（批准范围快照在请求行里，元数据只是展示与追溯）。
type poweropsApprovalGate struct{ store approvals.Store }

func newPoweropsApprovalGate(store approvals.Store) *poweropsApprovalGate {
	return &poweropsApprovalGate{store: store}
}

func (g *poweropsApprovalGate) CreateApproval(ctx context.Context, action powerops.Action, requestID, requestedBy, reason string, targetCount int) (string, error) {
	a, err := g.store.Create(&approvals.Approval{
		FunctionID: action.FunctionID(),
		Actor:      requestedBy,
		Mode:       "powerops",
		Route:      "/ops/power",
		Metadata: map[string]string{
			"powerRequestId": requestID,
			"action":         string(action),
			"targets":        strconv.Itoa(targetCount),
			"risk":           action.RiskWord(),
			"reason":         reason,
		},
	})
	if err != nil {
		return "", err
	}
	return a.ID, nil
}

func (g *poweropsApprovalGate) Approved(ctx context.Context, approvalID string) (bool, error) {
	a, err := g.store.Get(approvalID)
	if err != nil {
		return false, err
	}
	return a.State == "approved", nil
}

// RejectedApproval 报告审批记录是否已被拒绝（API 层惰性同步 pending→rejected 用）。
func (g *poweropsApprovalGate) RejectedApproval(approvalID string) (bool, error) {
	a, err := g.store.Get(approvalID)
	if err != nil {
		return false, err
	}
	return a.State == "rejected", nil
}

// agentOpsCommander 把 powerops.AgentCommander 适配到 agent-ops TCP 通道
// （logic/ops.AgentOpsClient.ExecuteCommand 同款 wire 协议；直接走 svc 持有的
// AgentSessionResolver，避免 svc→logic/ops→svc 环）。
type agentOpsCommander struct {
	resolver dispatch.AgentSessionResolver
}

func (c agentOpsCommander) Execute(ctx context.Context, agentID, command string, timeoutSeconds int) (string, error) {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 60
	}
	if c.resolver == nil {
		return "", fmt.Errorf("agent session resolver not configured")
	}
	caller, ok := c.resolver.ResolveAgentConn(agentID)
	if !ok {
		return "", fmt.Errorf("agent session not found: %s", agentID)
	}
	payload, err := proto.Marshal(&opsv1.ExecuteCommandRequest{
		Command:        command,
		TimeoutSeconds: int32(timeoutSeconds),
	})
	if err != nil {
		return "", fmt.Errorf("marshal execute request failed")
	}
	_, respData, err := caller.Call(ctx, protocol.MsgExecuteCommandRequest, payload)
	if err != nil {
		return "", err
	}
	resp := &opsv1.ExecuteCommandResponse{}
	if err := proto.Unmarshal(respData, resp); err != nil {
		return "", fmt.Errorf("unmarshal execute response failed")
	}
	if resp.Error != "" {
		return strings.TrimSpace(resp.StdOut), fmt.Errorf("%s", resp.Error)
	}
	return strings.TrimSpace(resp.StdOut), nil
}

// NodePowerResolver 从 nodes.meta.power 解析机器电源配置（机器清单挂现有
// Node 模型，design §2）。svc 装配与 api/ops 提交链共用同一解析源。
type NodePowerResolver struct{ DB *gorm.DB }

func (r NodePowerResolver) Resolve(nodeID string) (powerops.Machine, bool) {
	if r.DB == nil {
		return powerops.Machine{}, false
	}
	var row model.Node
	if err := r.DB.Where("node_id = ?", strings.TrimSpace(nodeID)).First(&row).Error; err != nil {
		return powerops.Machine{}, false
	}
	m, ok := MachineFromMeta(row.Meta)
	if !ok {
		return powerops.Machine{}, false
	}
	m.NodeID = row.NodeID
	return m, true
}

// MachineFromMeta 解析 nodes.meta.power 子对象；provider 缺失视为未登记。
func MachineFromMeta(meta datatypes.JSONMap) (powerops.Machine, bool) {
	raw, ok := meta["power"].(map[string]interface{})
	if !ok {
		return powerops.Machine{}, false
	}
	str := func(key string) string {
		v, _ := raw[key].(string)
		return strings.TrimSpace(v)
	}
	num := func(key string) int {
		switch v := raw[key].(type) {
		case float64:
			return int(v)
		case int:
			return v
		default:
			return 0
		}
	}
	m := powerops.Machine{
		Provider: str("provider"),
		BmcAddr:  str("bmcAddr"),
		BmcPort:  num("bmcPort"),
		MAC:      str("mac"),
		CloudRef: str("cloudRef"),
		AgentID:  str("agentId"),
	}
	if m.Provider == "" {
		return powerops.Machine{}, false
	}
	return m, true
}

// MachineToMeta 是 MachineFromMeta 的写入侧（JSONMap 键名同一来源）。
func MachineToMeta(m powerops.Machine) map[string]interface{} {
	out := map[string]interface{}{"provider": m.Provider}
	if m.BmcAddr != "" {
		out["bmcAddr"] = m.BmcAddr
	}
	if m.BmcPort > 0 {
		out["bmcPort"] = m.BmcPort
	}
	if m.MAC != "" {
		out["mac"] = m.MAC
	}
	if m.CloudRef != "" {
		out["cloudRef"] = m.CloudRef
	}
	if m.AgentID != "" {
		out["agentId"] = m.AgentID
	}
	return out
}

// powerRiskWord maps a risk word to the OpenAPI x-risk vocabulary
// (PolicyRiskFromContract mapping: safe→low, warning→medium, high→high).
func powerRiskWord(lowMediumHigh string) string {
	switch lowMediumHigh {
	case "low":
		return "safe"
	case "medium":
		return "warning"
	default:
		return "high"
	}
}

// registerPowerRiskDeclarations injects the ops.power.* x-risk declarations
// into the registry store — the sanctioned seam resolvePolicyRisk reads at
// approve time (OTP step-up). Without these, every ops.power approval would
// resolve medium and force_off would lose its OTP gate.
func registerPowerRiskDeclarations(store interface {
	UpsertOpenAPI(functionID string, operation *openapi3.Operation) error
}) {
	for _, a := range []powerops.Action{
		powerops.ActionPowerOn, powerops.ActionPowerOff, powerops.ActionForceOff,
		powerops.ActionReboot, powerops.ActionStatus,
	} {
		op := &openapi3.Operation{OperationID: a.FunctionID()}
		op.Extensions = map[string]interface{}{"x-risk": powerRiskWord(a.RiskWord())}
		if err := store.UpsertOpenAPI(a.FunctionID(), op); err != nil {
			slog.Default().Warn("powerops: register risk declaration failed", "functionId", a.FunctionID(), "error", err)
		}
	}
}

// newPoweropsOrchestrator 装配电源编排器（provision 同款：enabled=false 不装
// 配；provider 构建失败（如 ipmitool 缺失）记日志跳过该 kind，请求侧按
// provider 缺失诚实失败，绝不拖到批准后才报二进制缺失）。
func newPoweropsOrchestrator(ctx *ServiceContext) *powerops.Orchestrator {
	cfg := ctx.Config.Powerops
	providers := map[string]powerops.Provider{}

	if p, err := powerops.NewIPMIProvider(powerops.IPMIProvider{
		Binary:      cfg.IPMI.BinaryPath,
		User:        cfg.IPMI.User,
		PasswordEnv: cfg.IPMI.PasswordEnv,
		ExtraArgs:   cfg.IPMI.ExtraArgs,
	}); err == nil {
		providers[powerops.ProviderIPMI] = p
	} else {
		slog.Default().Warn("powerops: ipmi provider unavailable", "error", err)
	}
	providers[powerops.ProviderWOL] = powerops.NewWOLProvider()
	if cfg.Cloud.PowerOnCmd != "" || cfg.Cloud.StatusCmd != "" {
		providers[powerops.ProviderCloud] = &powerops.CloudProvider{
			Templates: map[powerops.Action]string{
				powerops.ActionPowerOn:  cfg.Cloud.PowerOnCmd,
				powerops.ActionPowerOff: cfg.Cloud.PowerOffCmd,
				powerops.ActionForceOff: cfg.Cloud.ForceOffCmd,
				powerops.ActionReboot:   cfg.Cloud.RebootCmd,
				powerops.ActionStatus:   cfg.Cloud.StatusCmd,
			},
			Runner: powerops.ExecCommandRunner{},
		}
	}
	providers[powerops.ProviderAgent] = &powerops.AgentProvider{Commander: agentOpsCommander{resolver: ctx.AgentSessionResolver}}

	if ctx.RegistryStore != nil {
		registerPowerRiskDeclarations(ctx.RegistryStore)
	}

	orch := &powerops.Orchestrator{
		Store:          powerops.NewGormStore(ctx.DB),
		Approval:       newPoweropsApprovalGate(ctx.ApprovalsStore),
		Providers:      providers,
		VerifyTimeout:  time.Duration(cfg.EffectiveVerifyTimeout()) * time.Millisecond,
		VerifyInterval: time.Duration(cfg.EffectiveVerifyInterval()) * time.Millisecond,
	}
	return orch
}
