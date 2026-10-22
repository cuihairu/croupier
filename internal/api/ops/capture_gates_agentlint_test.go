package ops

import (
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/capture/connector"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
)

// 发布校验服务端化（todo 4288）：依赖+能力感知 lint 覆盖。facts 经注册
// labels（captureTables / captureConnectorCapabilities），与 agent 侧
// buildPayload 上报面两侧锁死。

// withRegistryAgent 向 Service 注入带 labels 的已连 capture 会话。
func withRegistryAgent(s *Service, agentID string, labels map[string]string) {
	store := reg.NewStore()
	store.Mu().Lock()
	store.AgentsUnsafe()[agentID] = &reg.AgentSession{
		AgentID:  agentID,
		GameID:   "demo",
		Env:      "prod",
		Labels:   labels,
		ExpireAt: time.Now().Add(time.Hour),
	}
	store.Mu().Unlock()
	s.svcCtx.RegistryStore = store
}

func captureLabels(tables, caps string) map[string]string {
	return map[string]string{
		"role":                         "capture",
		"captureTables":                tables,
		"captureConnectorCapabilities": caps,
	}
}

func mustCaps(t *testing.T, label string) connector.Capabilities {
	t.Helper()
	caps, err := parseCapabilityLabel(label)
	if err != nil {
		t.Fatalf("parseCapabilityLabel(%q): %v", label, err)
	}
	return caps
}

func TestCaptureGatesPublishRejectsTableOutsideScope(t *testing.T) {
	s, ctx := newCaptureGatesService(t)
	withRegistryAgent(s, "cap-1", captureLabels("game.items", "afterImage,transactionMetadata,resumableCheckpoint"))

	// 对账流水表不在采集白名单：发布即拒（点名表名），不发布「永远不
	// 触发的规则」让运维误以为有防护。
	_, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: validGatesContent()})
	if err == nil || !strings.Contains(err.Error(), "game.ledger") || !strings.Contains(err.Error(), "outside the agent collection scope") {
		t.Fatalf("publish err = %v, want scope rejection naming game.ledger", err)
	}
}

func TestCaptureGatesPublishPassesWithinScope(t *testing.T) {
	s, ctx := newCaptureGatesService(t)
	withRegistryAgent(s, "cap-1", captureLabels("game.items,game.ledger", "afterImage,transactionMetadata,resumableCheckpoint"))

	resp, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: validGatesContent()})
	if err != nil {
		t.Fatalf("publish within scope: %v", err)
	}
	if resp.Version != 1 {
		t.Fatalf("version = %d", resp.Version)
	}
}

func TestCaptureGatesPublishRejectsMissingAfterImage(t *testing.T) {
	s, ctx := newCaptureGatesService(t)
	withRegistryAgent(s, "cap-1", captureLabels("game.items,game.ledger", "transactionMetadata,resumableCheckpoint"))

	_, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: validGatesContent()})
	if err == nil || !strings.Contains(err.Error(), "afterImage") {
		t.Fatalf("publish err = %v, want capability rejection naming afterImage", err)
	}

	// CEP 突变同样读行像取增量：能力缺失同拦。
	content := `{"cep":{"burst":[{"name":"amount-spike","table":"game.items","ops":["update"],"deltaField":"2","threshold":100,"mode":"single","severity":"warn"}]}}`
	if _, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: content}); err == nil || !strings.Contains(err.Error(), "afterImage") {
		t.Fatalf("burst publish err = %v, want afterImage rejection", err)
	}
}

func TestCaptureGatesPublishSkipsWhenNoAgentFacts(t *testing.T) {
	s, ctx := newCaptureGatesService(t)
	// 无 RegistryStore（裁剪部署）/无会话/会话过期：事实不可知=跳过 lint，
	// 结构+语义校验仍执行（滞后容忍，不惩罚未上报的存量部署）。
	if _, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: validGatesContent()}); err != nil {
		t.Fatalf("publish without facts: %v", err)
	}

	withRegistryAgent(s, "cap-1", map[string]string{"role": "capture"}) // 缺自报 labels
	if _, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: validGatesContent()}); err != nil {
		t.Fatalf("publish with labelless agent: %v", err)
	}

	withRegistryAgent(s, "cap-1", captureLabels("game.items", "warpDrive")) // 未知能力键
	if _, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: validGatesContent()}); err != nil {
		t.Fatalf("publish with unknown capability key: %v", err)
	}
}

func TestCaptureGatesCEPBurstTableOutsideScope(t *testing.T) {
	s, ctx := newCaptureGatesService(t)
	withRegistryAgent(s, "cap-1", captureLabels("game.items", "afterImage,transactionMetadata,resumableCheckpoint"))

	content := `{"cep":{"burst":[{"name":"ledger-spike","table":"game.ledger","ops":["insert"],"deltaField":"2","threshold":100,"mode":"single","severity":"warn"}]}}`
	_, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: content})
	if err == nil || !strings.Contains(err.Error(), "game.ledger") || !strings.Contains(err.Error(), "cep burst rule") {
		t.Fatalf("publish err = %v, want burst scope rejection", err)
	}
}

func TestLintAgainstAgentUnionAndIntersect(t *testing.T) {
	// 多 agent：tables 并集（任一覆盖即可）、capabilities 交集（从严——
	// 最弱连接器决定可发布面）。
	facts := captureAgentFacts{tables: map[string]bool{"game.items": true}, capabilities: mustCaps(t, "afterImage,transactionMetadata"), contributors: 1}
	other := captureAgentFacts{tables: map[string]bool{"game.ledger": true}, capabilities: mustCaps(t, "transactionMetadata,resumableCheckpoint"), contributors: 1}

	merged := captureAgentFacts{tables: map[string]bool{}, capabilities: facts.capabilities, contributors: 2}
	for tbl := range facts.tables {
		merged.tables[tbl] = true
	}
	for tbl := range other.tables {
		merged.tables[tbl] = true
	}
	merged.capabilities = intersectCapabilities(merged.capabilities, other.capabilities)
	if !merged.tables["game.items"] || !merged.tables["game.ledger"] {
		t.Fatalf("tables must union: %v", merged.tables)
	}
	// 交集 = {transactionMetadata}（afterImage 只有 agent1 有、
	// resumableCheckpoint 只有 agent2 有——最弱连接器决定可发布面）。
	if merged.capabilities.AfterImage || !merged.capabilities.TransactionMetadata || merged.capabilities.ResumableCheckpoint {
		t.Fatalf("capabilities must intersect to transactionMetadata only: %+v", merged.capabilities)
	}
}

func TestParseCapabilityLabelStrict(t *testing.T) {
	caps, err := parseCapabilityLabel("afterImage,resumableCheckpoint")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !caps.AfterImage || !caps.ResumableCheckpoint || caps.TransactionMetadata {
		t.Fatalf("caps = %+v", caps)
	}
	if _, err := parseCapabilityLabel("warpDrive"); err == nil || !strings.Contains(err.Error(), "warpDrive") {
		t.Fatalf("unknown key err = %v, want named rejection", err)
	}
}

// ValidateGates 语义真值在发布路径兜底（服务端化收口）：坏语义版本不再
// 「发布成功、全员 agent apply 拒绝滞留 last-good」。
func TestCaptureGatesPublishRunsSemanticValidation(t *testing.T) {
	s, ctx := newCaptureGatesService(t)
	bad := `{"reconcile":{"rules":[{"name":"bad-rule"}]}}` // 缺双表/键规格
	if _, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: bad}); err == nil {
		t.Fatal("semantically invalid content must be rejected at publish")
	}
}
