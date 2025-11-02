package tunnel

import (
    "context"
    "log/slog"
    "time"

    functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
    tunnelv1 "github.com/cuihairu/croupier/pkg/pb/croupier/tunnel/v1"
    localv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/local/v1"
    "github.com/cuihairu/croupier/internal/transport/interceptors"
    "google.golang.org/grpc"
)

type Client struct {
    addr     string
    agentID  string
    gameID   string
    env      string
    localAddr string // local function endpoint
}

var reconnects int64
func IncReconnect() { reconnects++ }
func Reconnects() int64 { return reconnects }

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
    // heartbeat sender
    done := make(chan struct{})
    go func(){
        ticker := time.NewTicker(15 * time.Second)
        defer ticker.Stop()
        for {
            select {
            case <-ticker.C:
                _ = stream.Send(&tunnelv1.TunnelMessage{Type:"heartbeat"})
            case <-done:
                return
            }
        }
    }()
    // recv loop (blocking)
    for {
        msg, err := stream.Recv()
        if err != nil { close(done); slog.Warn("tunnel recv end", "error", err.Error()); return err }
        if msg == nil { continue }
        // dial local (both function and local control share the same listener)
        lcc, err := grpc.Dial(c.localAddr, grpc.WithInsecure(), grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json")))
        if err != nil { continue }
        lcli := functionv1.NewFunctionServiceClient(lcc)
        lctrl := localv1.NewLocalControlServiceClient(lcc)
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
            case "list_local_req":
                lr := msg.ListReq
                // fetch local instances and respond service_ids for the function
                resp, err := lctrl.ListLocal(ctx, &localv1.ListLocalRequest{})
                if err != nil || resp == nil {
                    _ = stream.Send(&tunnelv1.TunnelMessage{Type:"list_local_res", ListRes: &tunnelv1.ListLocalResponse{RequestId: lr.RequestId, FunctionId: lr.FunctionId, Error: "list failed"}})
                } else {
                    var ids []string
                    for _, lf := range resp.Functions { if lf.Id == lr.FunctionId { for _, inst := range lf.Instances { ids = append(ids, inst.ServiceId) } } }
                    _ = stream.Send(&tunnelv1.TunnelMessage{Type:"list_local_res", ListRes: &tunnelv1.ListLocalResponse{RequestId: lr.RequestId, FunctionId: lr.FunctionId, ServiceIds: ids}})
                }
            case "get_job_result_req":
                jr := msg.JobResReq
                r, err := lctrl.GetJobResult(ctx, &localv1.GetJobResultRequest{JobId: jr.JobId})
                if err != nil || r == nil {
                    _ = stream.Send(&tunnelv1.TunnelMessage{Type:"get_job_result_res", JobResRes: &tunnelv1.GetJobResultResponse{RequestId: jr.RequestId, State: "unknown", Error: "query failed"}})
                } else {
                    _ = stream.Send(&tunnelv1.TunnelMessage{Type:"get_job_result_res", JobResRes: &tunnelv1.GetJobResultResponse{RequestId: jr.RequestId, State: r.State, Payload: r.Payload, Error: r.Error}})
                }
            }
        _ = lcc.Close()
    }
}
