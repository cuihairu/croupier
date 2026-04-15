package agent

import (
	"bytes"
	"context"
	"net"
	"strings"
	"testing"
	"time"

	controlnng "github.com/cuihairu/croupier/internal/nng"
	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/rep"
	_ "go.nanomsg.org/mangos/v3/transport/tcp"
	"google.golang.org/protobuf/proto"
)

type testProviderManager struct {
	prefix string
}

func (m *testProviderManager) IsPlatformFunction(functionID string) bool {
	return strings.HasPrefix(functionID, m.prefix+".")
}

func (m *testProviderManager) Call(ctx context.Context, functionID string, request []byte) ([]byte, error) {
	return append([]byte(nil), request...), nil
}

func TestAgentRegisterAndServerInvokeRoundTrip(t *testing.T) {
	t.Parallel()

	controlAddr := reserveTCPAddr(t)
	agentAddr := reserveTCPAddr(t)

	registryStore := reg.NewStore()
	controlServer := controlnng.NewServer(controlAddr, registryStore)
	if err := controlServer.Start(); err != nil {
		t.Fatalf("start control server failed: %v", err)
	}
	defer func() {
		if err := controlServer.Stop(); err != nil {
			t.Fatalf("stop control server failed: %v", err)
		}
	}()

	localStore := agentlocal.NewLocalStore()
	localStore.Register("provider:test", "", "1.0.0", []*sdkv1.LocalFunctionDescriptor{
		{
			Id:           "test.echo",
			Version:      "1.0.0",
			Summary:      "Echo payload",
			Description:  "Returns request payload as-is",
			OperationId:  "echoPayload",
			InputSchema:  `{"type":"object","properties":{"message":{"type":"string"}},"required":["message"]}`,
			OutputSchema: `{"type":"object","properties":{"message":{"type":"string"}}}`,
			Category:     "testing",
			Entity:       "RoundTrip",
			Operation:    "invoke",
		},
	})

	agentServer := controlnng.NewAgentServer(agentAddr, localStore)
	agentServer.SetProviderManager(&testProviderManager{prefix: "test"})
	if err := agentServer.Start(); err != nil {
		t.Fatalf("start agent server failed: %v", err)
	}
	defer func() {
		if err := agentServer.Stop(); err != nil {
			t.Fatalf("stop agent server failed: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	upstream := NewUpstreamClient(controlAddr, "agent-roundtrip", localStore, &UpstreamMetadata{
		GameID:  "game-1",
		Env:     "dev",
		Version: "1.0.0",
		RPCAddr: agentAddr,
	})
	defer upstream.Stop()

	if err := upstream.dialServer(ctx); err != nil {
		t.Fatalf("dialServer failed: %v", err)
	}

	waitForCondition(t, 3*time.Second, func() bool {
		registryStore.Mu().RLock()
		defer registryStore.Mu().RUnlock()
		agent := registryStore.AgentsUnsafe()["agent-roundtrip"]
		if agent == nil {
			return false
		}
		meta, ok := agent.Functions["test.echo"]
		return ok && meta.Enabled && agent.RPCAddr == agentAddr
	})

	op, err := registryStore.GetOpenAPI("test.echo")
	if err != nil {
		t.Fatalf("expected openapi operation for test.echo: %v", err)
	}
	if op.RequestBody == nil || op.RequestBody.Value == nil {
		t.Fatal("expected request body schema to be synced to server")
	}
	if op.Responses == nil || op.Responses.Value("200") == nil {
		t.Fatal("expected response schema to be synced to server")
	}

	dispatcher := dispatch.NewDispatcher(registryStore)
	defer func() {
		if err := dispatcher.Close(); err != nil {
			t.Fatalf("close dispatcher failed: %v", err)
		}
	}()

	payload := []byte(`{"message":"hello roundtrip"}`)
	resp, err := dispatcher.Invoke(ctx, "test.echo", payload)
	if err != nil {
		t.Fatalf("dispatcher invoke failed: %v", err)
	}
	if !bytes.Equal(resp, payload) {
		t.Fatalf("unexpected invoke response: got %s want %s", string(resp), string(payload))
	}
}

func TestSDKRegisterToAgentAndServerInvokeRoundTrip(t *testing.T) {
	t.Parallel()

	controlAddr := reserveTCPAddr(t)
	agentAddr := reserveTCPAddr(t)
	sdkAddr := reserveTCPAddr(t)

	controlServer := controlnng.NewServer(controlAddr, nil)
	if err := controlServer.Start(); err != nil {
		t.Fatalf("start control server failed: %v", err)
	}
	defer func() {
		if err := controlServer.Stop(); err != nil {
			t.Fatalf("stop control server failed: %v", err)
		}
	}()

	sdkPayload := []byte(`{"message":"hello from sdk"}`)
	sdkServer := startSDKInvokeServer(t, sdkAddr, sdkPayload)
	defer sdkServer.Close()

	localStore := agentlocal.NewLocalStore()
	agentServer := controlnng.NewAgentServer(agentAddr, localStore)
	if err := agentServer.Start(); err != nil {
		t.Fatalf("start agent server failed: %v", err)
	}
	defer func() {
		if err := agentServer.Stop(); err != nil {
			t.Fatalf("stop agent server failed: %v", err)
		}
	}()

	registerLocalToAgent(t, agentAddr, sdkAddr, "sdk-echo", []*sdkv1.LocalFunctionDescriptor{
		{
			Id:           "sdk.echo",
			Version:      "1.0.0",
			Summary:      "SDK echo",
			Description:  "Echoes from SDK server",
			OperationId:  "sdkEcho",
			InputSchema:  `{"type":"object","properties":{"message":{"type":"string"}}}`,
			OutputSchema: `{"type":"object","properties":{"message":{"type":"string"}}}`,
			Category:     "sdk",
			Entity:       "SDK",
			Operation:    "invoke",
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	upstream := NewUpstreamClient(controlAddr, "agent-sdk-roundtrip", localStore, &UpstreamMetadata{
		GameID:  "game-1",
		Env:     "dev",
		Version: "1.0.0",
		RPCAddr: agentAddr,
	})
	defer upstream.Stop()

	if err := upstream.dialServer(ctx); err != nil {
		t.Fatalf("dialServer failed: %v", err)
	}

	waitForCondition(t, 3*time.Second, func() bool {
		controlServer.Store().Mu().RLock()
		defer controlServer.Store().Mu().RUnlock()
		agent := controlServer.Store().AgentsUnsafe()["agent-sdk-roundtrip"]
		if agent == nil {
			return false
		}
		_, ok := agent.Functions["sdk.echo"]
		if !ok {
			return false
		}
		for _, provider := range agent.Providers {
			if provider.ProviderID == "sdk-echo" {
				return true
			}
		}
		return false
	})

	dispatcher := dispatch.NewDispatcher(controlServer.Store())
	defer func() {
		if err := dispatcher.Close(); err != nil {
			t.Fatalf("close dispatcher failed: %v", err)
		}
	}()

	resp, err := dispatcher.Invoke(ctx, "sdk.echo", []byte(`{"message":"ignored by fake sdk"}`))
	if err != nil {
		t.Fatalf("dispatcher invoke failed: %v", err)
	}
	if !bytes.Equal(resp, sdkPayload) {
		t.Fatalf("unexpected sdk invoke response: got %s want %s", string(resp), string(sdkPayload))
	}
}

func reserveTCPAddr(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve tcp addr failed: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("close reserved listener failed: %v", err)
	}
	return addr
}

func waitForCondition(t *testing.T, timeout time.Duration, check func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

func registerLocalToAgent(t *testing.T, agentAddr, rpcAddr, serviceID string, functions []*sdkv1.LocalFunctionDescriptor) {
	t.Helper()

	client := controlnng.NewClient(agentAddr)
	if err := client.Dial(); err != nil {
		t.Fatalf("dial agent failed: %v", err)
	}
	defer client.Close()

	req := &sdkv1.RegisterLocalRequest{
		ServiceId: serviceID,
		Version:   "1.0.0",
		RpcAddr:   rpcAddr,
		Functions: functions,
	}
	body := sdkv1.MarshalRegisterLocalRequest(req)
	if _, err := client.Call(context.Background(), protocol.MsgRegisterLocalRequest, body); err != nil {
		t.Fatalf("register local to agent failed: %v", err)
	}
}

func startSDKInvokeServer(t *testing.T, addr string, payload []byte) mangos.Socket {
	t.Helper()

	sock, err := rep.NewSocket()
	if err != nil {
		t.Fatalf("create sdk rep socket failed: %v", err)
	}
	if err := sock.SetOption(mangos.OptionRecvDeadline, time.Second); err != nil {
		t.Fatalf("set recv deadline failed: %v", err)
	}
	if err := sock.Listen("tcp://" + addr); err != nil {
		t.Fatalf("sdk server listen failed: %v", err)
	}

	go func() {
		for {
			msg, err := sock.RecvMsg()
			if err != nil {
				return
			}
			_, msgID, reqID, _, err := protocol.ParseMessageFromBody(msg.Body)
			msg.Free()
			if err != nil {
				return
			}
			if msgID != protocol.MsgInvokeRequest {
				return
			}
			respBytes, err := proto.Marshal(&sdkv1.InvokeResponse{Payload: payload})
			if err != nil {
				return
			}
			respMsg := mangos.NewMessage(0)
			respMsg.Body = protocol.NewMessageBody(protocol.MsgInvokeResponse, reqID, respBytes)
			if err := sock.SendMsg(respMsg); err != nil {
				return
			}
		}
	}()

	return sock
}
