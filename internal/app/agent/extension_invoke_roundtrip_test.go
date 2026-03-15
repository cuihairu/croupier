package agent

import (
	"context"
	"testing"
	"time"

	extensionsync "github.com/cuihairu/croupier/internal/core/extension/sync"
	controlnng "github.com/cuihairu/croupier/internal/nng"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

func TestExtensionFunctionInvokeThroughAgentServer(t *testing.T) {
	t.Parallel()

	agentAddr := reserveTCPAddr(t)
	app := New("", "agent-ext-invoke")
	app.SetNNGAddr(agentAddr)
	if err := app.StartNNGServer(); err != nil {
		t.Fatalf("start agent nng server failed: %v", err)
	}
	defer app.Stop()

	_, err := app.ApplyExtensionSyncPayload(&extensionsync.AgentSyncPayload{
		AgentID:     "agent-ext-invoke",
		GeneratedAt: time.Now().Unix(),
		Version:     "v1",
		Installations: []extensionsync.AgentInstallationPayload{
			{
				InstallationID: 10,
				ExtensionID:    "official.analytics",
				ReleaseVersion: "1.0.0",
				ConfigJSON:     `{}`,
				Bindings: []extensionsync.AgentBindingPayload{
					{
						BindingType: "function",
						BindingKey:  "analytics.query",
						SpecJSON:    `{"driver":"workflow-driver"}`,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("apply extension payload failed: %v", err)
	}

	client := controlnng.NewClient(agentAddr)
	if err := client.Dial(); err != nil {
		t.Fatalf("dial agent failed: %v", err)
	}
	defer client.Close()

	req := &sdkv1.InvokeRequest{
		FunctionId: "analytics.query",
		Payload:    []byte(`{"message":"hello extension"}`),
	}
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal invoke request failed: %v", err)
	}
	respBytes, err := client.Call(context.Background(), protocol.MsgInvokeRequest, reqBytes)
	if err != nil {
		t.Fatalf("invoke extension function failed: %v", err)
	}
	resp := &sdkv1.InvokeResponse{}
	if err := proto.Unmarshal(respBytes, resp); err != nil {
		t.Fatalf("unmarshal invoke response failed: %v", err)
	}
	if string(resp.GetPayload()) != `{"message":"hello extension"}` {
		t.Fatalf("unexpected extension invoke response payload: %s", string(resp.GetPayload()))
	}
}
