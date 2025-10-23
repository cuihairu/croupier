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
            if msg == nil { continue }
            // dial local
            lcc, err := grpc.Dial(c.localAddr, grpc.WithInsecure(), grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json")))
            if err != nil { continue }
            lcli := functionv1.NewFunctionServiceClient(lcc)
            switch msg.Type {
            case "invoke":
                inv := msg.Invoke
                resp, err := lcli.Invoke(ctx, &functionv1.InvokeRequest{FunctionId: inv.FunctionId, IdempotencyKey: inv.IdempotencyKey, Payload: inv.Payload, Metadata: inv.Metadata})
                if err != nil {
                    _ = stream.Send(&tunnelv1.TunnelMessage{Type:"result", Result: &tunnelv1.ResultFrame{RequestId: inv.RequestId, Error: err.Error()}})
                } else {
                    _ = stream.Send(&tunnelv1.TunnelMessage{Type:"result", Result: &tunnelv1.ResultFrame{RequestId: inv.RequestId, Payload: resp.GetPayload()}})
                }
            case "start_job":
                st := msg.Start
                resp, err := lcli.StartJob(ctx, &functionv1.InvokeRequest{FunctionId: st.FunctionId, IdempotencyKey: st.IdempotencyKey, Payload: st.Payload, Metadata: st.Metadata})
                if err != nil {
                    _ = stream.Send(&tunnelv1.TunnelMessage{Type:"start_job_result", StartR: &tunnelv1.StartJobResult{RequestId: st.RequestId, Error: err.Error()}})
                } else {
                    jobID := resp.GetJobId()
                    _ = stream.Send(&tunnelv1.TunnelMessage{Type:"start_job_result", StartR: &tunnelv1.StartJobResult{RequestId: st.RequestId, JobId: jobID}})
                    // stream events
                    go func(job string){
                        sj, err := lcli.StreamJob(ctx, &functionv1.JobStreamRequest{JobId: job})
                        if err != nil { return }
                        for {
                            ev, err := sj.Recv()
                            if err != nil { return }
                            _ = stream.Send(&tunnelv1.TunnelMessage{Type:"job_event", JobEvt: &tunnelv1.JobEventFrame{JobId: job, Type: ev.GetType(), Message: ev.GetMessage(), Progress: ev.GetProgress(), Payload: ev.GetPayload()}})
                            if ev.GetType()=="done" || ev.GetType()=="error" { return }
                        }
                    }(jobID)
                }
            case "cancel_job":
                cj := msg.Cancel
                _, _ = lcli.CancelJob(ctx, &functionv1.CancelJobRequest{JobId: cj.JobId})
                // no ack in PoC
            }
            _ = lcc.Close()
        }
    }()
    return nil
}
