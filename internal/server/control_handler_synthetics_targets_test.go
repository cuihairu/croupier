package server

import (
	"context"
	"testing"

	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

func mustMarshalSynTargetsReq(t *testing.T, req *opsv1.SyntheticsTargetsConfigRequest) []byte {
	t.Helper()
	data, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}

func mustUnmarshalSynTargetsResp(t *testing.T, data []byte) *opsv1.SyntheticsTargetsConfigResponse {
	t.Helper()
	resp := &opsv1.SyntheticsTargetsConfigResponse{}
	if err := proto.Unmarshal(data, resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return resp
}

func mustRunSynTargets(t *testing.T, svc *ControlService, agentID string) []byte {
	t.Helper()
	data, err := svc.handleSyntheticsTargetsConfig(context.Background(), mustMarshalSynTargetsReq(t, &opsv1.SyntheticsTargetsConfigRequest{AgentId: agentID}))
	if err != nil {
		t.Fatalf("handleSyntheticsTargetsConfig: %v", err)
	}
	return data
}

func TestHandleSyntheticsTargetsConfig_PassesThroughLatestVersion(t *testing.T) {
	svc, db := newConfigTestService(t)
	seedCaptureAgent(t, svc, "syn-01", "demo", "prod")
	targetsJSON := `[{"name":"web","url":"http://127.0.0.1","timeout":"3s","expectedStatus":200,"targetCategory":"game"}]`
	row := configVersionRow("synthetics.targets", "demo", "prod", 4, targetsJSON)
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	resp := mustUnmarshalSynTargetsResp(t, mustRunSynTargets(t, svc, "syn-01"))
	if resp.GetVersion() != "4" || resp.GetTargetsJson() != targetsJSON {
		t.Fatalf("resp = %+v, want version 4 with targets json", resp)
	}
}

func TestHandleSyntheticsTargetsConfig_EmptyWhenNotPublished(t *testing.T) {
	svc, _ := newConfigTestService(t)
	seedCaptureAgent(t, svc, "syn-01", "demo", "prod")
	resp := mustUnmarshalSynTargetsResp(t, mustRunSynTargets(t, svc, "syn-01"))
	if resp.GetVersion() != "" || resp.GetTargetsJson() != "" {
		t.Fatalf("resp = %+v, want empty (keep local targets)", resp)
	}
}

func TestHandleSyntheticsTargetsConfig_ScopeIsolatedBySession(t *testing.T) {
	svc, db := newConfigTestService(t)
	seedCaptureAgent(t, svc, "syn-01", "demo", "prod")
	row := configVersionRow("synthetics.targets", "demo", "dev", 9, `[]`)
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	resp := mustUnmarshalSynTargetsResp(t, mustRunSynTargets(t, svc, "syn-01"))
	if resp.GetVersion() != "" {
		t.Fatalf("resp = %+v, want empty for other env", resp)
	}
}

func TestHandleSyntheticsTargetsConfig_UnknownAgentAndNilModel(t *testing.T) {
	svc, _ := newConfigTestService(t)
	if _, err := svc.handleSyntheticsTargetsConfig(context.Background(), mustMarshalSynTargetsReq(t, &opsv1.SyntheticsTargetsConfigRequest{AgentId: "ghost"})); err == nil {
		t.Fatal("unknown agent must error")
	}
	if _, err := svc.handleSyntheticsTargetsConfig(context.Background(), mustMarshalSynTargetsReq(t, &opsv1.SyntheticsTargetsConfigRequest{AgentId: ""})); err == nil {
		t.Fatal("empty agent id must error")
	}
	svc.SetConfigVersionModel(nil)
	seedCaptureAgent(t, svc, "syn-01", "demo", "prod")
	if _, err := svc.handleSyntheticsTargetsConfig(context.Background(), mustMarshalSynTargetsReq(t, &opsv1.SyntheticsTargetsConfigRequest{AgentId: "syn-01"})); err == nil {
		t.Fatal("nil config model must error")
	}
}

func TestHandleSyntheticsTargetsConfig_ResolverFailureReturnsEmpty(t *testing.T) {
	svc, _ := newConfigTestService(t)
	seedCaptureAgent(t, svc, "syn-01", "demo", "prod")
	svc.SetGameDBResolver(&fakeGameDBResolver{err: errDBDown})
	resp := mustUnmarshalSynTargetsResp(t, mustRunSynTargets(t, svc, "syn-01"))
	if resp.GetVersion() != "" || resp.GetTargetsJson() != "" {
		t.Fatalf("resp = %+v, want empty on resolver failure", resp)
	}
}

func TestSyntheticsTargetsMsgIDPair(t *testing.T) {
	if protocol.MsgSyntheticsTargetsConfigResponse != protocol.MsgSyntheticsTargetsConfigRequest+1 {
		t.Fatal("synthetics targets response must be request+1")
	}
	if !protocol.IsRequest(protocol.MsgSyntheticsTargetsConfigRequest) || !protocol.IsResponse(protocol.MsgSyntheticsTargetsConfigResponse) {
		t.Fatal("synthetics targets msg id parity broken")
	}
}
