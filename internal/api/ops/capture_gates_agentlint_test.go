package ops

import (
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/capture/connector"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
)

// Server-side publish validation (todo 4288): dependency- and
// capability-aware lint coverage. The facts flow through the registration
// labels (captureTables / captureConnectorCapabilities), locked on both sides
// with the agent-side buildPayload reporting surface.

// withRegistryAgent injects a connected capture session with labels into the
// Service.
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

	// The ledger transaction table is outside the collection whitelist:
	// publish is rejected outright (naming the table) instead of publishing a
	// "rule that never triggers" and lulling ops into a false sense of
	// protection.
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

	// CEP mutations likewise read row images like deltas: missing
	// capabilities are blocked the same way.
	content := `{"cep":{"burst":[{"name":"amount-spike","table":"game.items","ops":["update"],"deltaField":"2","threshold":100,"mode":"single","severity":"warn"}]}}`
	if _, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: content}); err == nil || !strings.Contains(err.Error(), "afterImage") {
		t.Fatalf("burst publish err = %v, want afterImage rejection", err)
	}
}

func TestCaptureGatesPublishSkipsWhenNoAgentFacts(t *testing.T) {
	s, ctx := newCaptureGatesService(t)
	// No RegistryStore (trimmed deployment) / no session / expired session:
	// facts unknown = skip the lint, while structural + semantic validation
	// still runs (lag tolerance, so pre-existing deployments that never
	// reported facts are not punished).
	if _, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: validGatesContent()}); err != nil {
		t.Fatalf("publish without facts: %v", err)
	}

	withRegistryAgent(s, "cap-1", map[string]string{"role": "capture"}) // missing self-reported labels
	if _, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: validGatesContent()}); err != nil {
		t.Fatalf("publish with labelless agent: %v", err)
	}

	withRegistryAgent(s, "cap-1", captureLabels("game.items", "warpDrive")) // unknown capability key
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
	// Multiple agents: tables take the union (any one covering is enough),
	// capabilities take the intersection (strict -- the weakest connector
	// determines the publishable surface).
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
	// Intersection = {transactionMetadata} (only agent1 has afterImage and
	// only agent2 has resumableCheckpoint -- the weakest connector determines
	// the publishable surface).
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

// ValidateGates semantic truth is enforced as a fallback on the publish path
// (the close-out of server-side validation): a bad semantic version no longer
// "publishes successfully, then every agent apply rejects it and everyone
// lingers on last-good".
func TestCaptureGatesPublishRunsSemanticValidation(t *testing.T) {
	s, ctx := newCaptureGatesService(t)
	bad := `{"reconcile":{"rules":[{"name":"bad-rule"}]}}` // missing dual-table/key specs
	if _, err := s.CaptureGatesPut(ctx, &CaptureGatesPutRequest{Content: bad}); err == nil {
		t.Fatal("semantically invalid content must be rejected at publish")
	}
}
