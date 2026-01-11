package agent

import (
	"context"
	"testing"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	localv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/local/v1"
	serverv1 "github.com/cuihairu/croupier/pkg/pb/croupier/server/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type fakeControlClient struct {
	lastRegister *serverv1.RegisterRequest
}

func (f *fakeControlClient) ListFunctionsSummary(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*serverv1.ListFunctionsSummaryResponse, error) {
	return &serverv1.ListFunctionsSummaryResponse{}, nil
}

func (f *fakeControlClient) Register(ctx context.Context, in *serverv1.RegisterRequest, opts ...grpc.CallOption) (*serverv1.RegisterResponse, error) {
	f.lastRegister = in
	return &serverv1.RegisterResponse{}, nil
}

func (f *fakeControlClient) Heartbeat(ctx context.Context, in *serverv1.HeartbeatRequest, opts ...grpc.CallOption) (*serverv1.HeartbeatResponse, error) {
	return &serverv1.HeartbeatResponse{}, nil
}

func (f *fakeControlClient) RegisterCapabilities(ctx context.Context, in *serverv1.RegisterCapabilitiesRequest, opts ...grpc.CallOption) (*serverv1.RegisterCapabilitiesResponse, error) {
	return &serverv1.RegisterCapabilitiesResponse{}, nil
}

func TestUpstreamClient_SyncBuildsRequestFromStore(t *testing.T) {
	t.Parallel()

	store := agentlocal.NewLocalStore()
	store.Register("svc-1", "127.0.0.1:10001", "sv1", []*localv1.LocalFunctionDescriptor{
		{Id: "f1", Version: "1.0.0"},
		{Id: "f2", Version: "2.0.0"},
	})

	client := NewUpstreamClient("127.0.0.1:9999", "agent-1", store, &UpstreamMetadata{
		GameID:  "game-1",
		Env:     "staging",
		Version: "agent-ver",
		RPCAddr: "127.0.0.1:19090",
	})

	fake := &fakeControlClient{}
	client.client = fake

	if err := client.Sync(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.lastRegister == nil {
		t.Fatal("expected register request")
	}
	if fake.lastRegister.AgentId != "agent-1" || fake.lastRegister.GameId != "game-1" || fake.lastRegister.Env != "staging" || fake.lastRegister.RpcAddr != "127.0.0.1:19090" || fake.lastRegister.Version != "agent-ver" {
		t.Fatalf("unexpected metadata in request: %+v", fake.lastRegister)
	}
	if len(fake.lastRegister.Functions) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(fake.lastRegister.Functions))
	}

	funcByID := map[string]*serverv1.FunctionDescriptor{}
	for _, fn := range fake.lastRegister.Functions {
		funcByID[fn.GetId()] = fn
	}

	if fn := funcByID["f1"]; fn == nil || fn.GetVersion() != "1.0.0" || !fn.GetEnabled() {
		t.Fatalf("unexpected f1: %+v", fn)
	}
	if fn := funcByID["f2"]; fn == nil || fn.GetVersion() != "2.0.0" || !fn.GetEnabled() {
		t.Fatalf("unexpected f2: %+v", fn)
	}
}
