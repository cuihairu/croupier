package tunnel

import (
    "context"
    "log"

    functionv1 "github.com/your-org/croupier/gen/go/croupier/function/v1"
    tunnelv1 "github.com/your-org/croupier/gen/go/croupier/tunnel/v1"
    "github.com/your-org/croupier/internal/transport/interceptors"
    "google.golang.org/grpc"
)

type Client struct {
    addr     string
    agentID  string
    gameID   string
    env      string
    localAddr string // local function endpoint
}

func NewClient(addr, agentID, gameID, env, localAddr string) *Client {
    return &Client{addr: addr, agentID: agentID, gameID: gameID, env: env, localAddr: localAddr}
}

func (c *Client) Start(ctx context.Context) error {
    base := []grpc.DialOption{grpc.WithInsecure(), grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json"))}
    opts := append(base, interceptors.Chain(nil)...)
    cc, err := grpc.DialContext(ctx, c.addr, opts...)
    if err != nil { return err }
    cli := tunnelv1.NewTunnelServiceClient(cc)
    stream, err := cli.Open(ctx)
    if err != nil { return err }
    // send hello
    if err := stream.Send(&tunnelv1.TunnelMessage{Type: "hello", Hello: &tunnelv1.Hello{AgentId: c.agentID, GameId: c.gameID, Env: c.env}}); err != nil { return err }
    // recv loop
    go func(){
        for {
            msg, err := stream.Recv()
            if err != nil { log.Printf("tunnel recv end: %v", err); return }
            if msg == nil || msg.Type != "invoke" || msg.Invoke == nil { continue }
            inv := msg.Invoke
            // forward to local function endpoint
            // best-effort dial local
            lcc, err := grpc.Dial(c.localAddr, grpc.WithInsecure(), grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json")))
            if err != nil { _ = stream.Send(&tunnelv1.TunnelMessage{Type:"result", Result: &tunnelv1.ResultFrame{RequestId: inv.RequestId, Error: err.Error()}}); continue }
            cli := functionv1.NewFunctionServiceClient(lcc)
            resp, err := cli.Invoke(ctx, &functionv1.InvokeRequest{FunctionId: inv.FunctionId, IdempotencyKey: inv.IdempotencyKey, Payload: inv.Payload, Metadata: inv.Metadata})
            _ = lcc.Close()
            if err != nil {
                _ = stream.Send(&tunnelv1.TunnelMessage{Type:"result", Result: &tunnelv1.ResultFrame{RequestId: inv.RequestId, Error: err.Error()}})
                continue
            }
            _ = stream.Send(&tunnelv1.TunnelMessage{Type:"result", Result: &tunnelv1.ResultFrame{RequestId: inv.RequestId, Payload: resp.GetPayload()}})
        }
    }()
    return nil
}

