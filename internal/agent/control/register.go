package control

import (
    "context"
    "log"
    "time"

    "google.golang.org/grpc"

    controlv1 "github.com/your-org/croupier/gen/go/croupier/control/v1"
)

// Client wraps the generated ControlService client with helper methods.
type Client struct {
    c controlv1.ControlServiceClient
}

func NewClient(cc *grpc.ClientConn) *Client { return &Client{c: controlv1.NewControlServiceClient(cc)} }

// RegisterAndHeartbeat performs initial register and keeps sending heartbeats until ctx done.
func (cl *Client) RegisterAndHeartbeat(ctx context.Context, agentID, version, rpcAddr, gameID, env string, fns []*controlv1.FunctionDescriptor) {
    resp, err := cl.c.Register(ctx, &controlv1.RegisterRequest{AgentId: agentID, Version: version, RpcAddr: rpcAddr, GameId: gameID, Env: env, Functions: fns})
    if err != nil {
        log.Printf("register failed: %v", err)
        return
    }
    log.Printf("registered session=%s", resp.GetSessionId())
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if _, err := cl.c.Heartbeat(ctx, &controlv1.HeartbeatRequest{AgentId: agentID, SessionId: resp.GetSessionId()}); err != nil {
                log.Printf("heartbeat failed: %v", err)
            }
        }
    }
}
