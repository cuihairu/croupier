package tunnelv1

import (
    "context"
    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

type Hello struct {
    AgentId string `json:"agent_id,omitempty"`
    GameId  string `json:"game_id,omitempty"`
    Env     string `json:"env,omitempty"`
}

type InvokeFrame struct {
    RequestId      string            `json:"request_id,omitempty"`
    FunctionId     string            `json:"function_id,omitempty"`
    IdempotencyKey string            `json:"idempotency_key,omitempty"`
    Payload        []byte            `json:"payload,omitempty"`
    Metadata       map[string]string `json:"metadata,omitempty"`
}

type ResultFrame struct {
    RequestId string `json:"request_id,omitempty"`
    Payload   []byte `json:"payload,omitempty"`
    Error     string `json:"error,omitempty"`
}

type TunnelMessage struct {
    Type    string            `json:"type,omitempty"` // "hello"|"invoke"|"result"|"start_job"|"start_job_result"|"job_event"|"cancel_job"|"cancel_job_result"|"heartbeat"
    Hello   *Hello            `json:"hello,omitempty"`
    Invoke  *InvokeFrame      `json:"invoke,omitempty"`
    Result  *ResultFrame      `json:"result,omitempty"`
    Start   *StartJobFrame    `json:"start_job,omitempty"`
    StartR  *StartJobResult   `json:"start_job_result,omitempty"`
    JobEvt  *JobEventFrame    `json:"job_event,omitempty"`
    Cancel  *CancelJobFrame   `json:"cancel_job,omitempty"`
    CancelR *CancelJobResult  `json:"cancel_job_result,omitempty"`
}
// --- StartJob/Cancel/Events ---
type StartJobFrame struct {
    RequestId      string            `json:"request_id,omitempty"`
    FunctionId     string            `json:"function_id,omitempty"`
    IdempotencyKey string            `json:"idempotency_key,omitempty"`
    Payload        []byte            `json:"payload,omitempty"`
    Metadata       map[string]string `json:"metadata,omitempty"`
}
type StartJobResult struct {
    RequestId string `json:"request_id,omitempty"`
    JobId     string `json:"job_id,omitempty"`
    Error     string `json:"error,omitempty"`
}
type JobEventFrame struct {
    JobId    string `json:"job_id,omitempty"`
    Type     string `json:"type,omitempty"`
    Message  string `json:"message,omitempty"`
    Progress int32  `json:"progress,omitempty"`
    Payload  []byte `json:"payload,omitempty"`
}
type CancelJobFrame struct { JobId string `json:"job_id,omitempty"` }
type CancelJobResult struct { RequestId string `json:"request_id,omitempty"`; JobId string `json:"job_id,omitempty"`; Error string `json:"error,omitempty"` }

// extend message union
// (note: above fields are included directly in TunnelMessage)


type TunnelServiceServer interface {
    Open(TunnelService_OpenServer) error
}

type UnimplementedTunnelServiceServer struct{}
func (*UnimplementedTunnelServiceServer) Open(TunnelService_OpenServer) error { return status.Errorf(codes.Unimplemented, "method Open not implemented") }

func RegisterTunnelServiceServer(s *grpc.Server, srv TunnelServiceServer){
    s.RegisterService(&grpc.ServiceDesc{
        ServiceName: "croupier.tunnel.v1.TunnelService",
        HandlerType: (*TunnelServiceServer)(nil),
        Streams: []grpc.StreamDesc{{StreamName:"Open", Handler: _TunnelService_Open_Handler, ServerStreams:true, ClientStreams:true}},
        Methods: []grpc.MethodDesc{},
    }, srv)
}

type TunnelService_OpenServer interface { Send(*TunnelMessage) error; Recv() (*TunnelMessage, error); grpc.ServerStream }

func _TunnelService_Open_Handler(srv interface{}, stream grpc.ServerStream) error {
    return srv.(TunnelServiceServer).Open(&tunnelServiceOpenServer{stream})
}

type tunnelServiceOpenServer struct{ grpc.ServerStream }
func (x *tunnelServiceOpenServer) Send(m *TunnelMessage) error { return x.ServerStream.SendMsg(m) }
func (x *tunnelServiceOpenServer) Recv() (*TunnelMessage, error) { m := new(TunnelMessage); if err := x.ServerStream.RecvMsg(m); err != nil { return nil, err }; return m, nil }

// Client
type TunnelServiceClient interface { Open(ctx context.Context, opts ...grpc.CallOption) (TunnelService_OpenClient, error) }
type tunnelServiceClient struct{ cc grpc.ClientConnInterface }
func NewTunnelServiceClient(cc grpc.ClientConnInterface) TunnelServiceClient { return &tunnelServiceClient{cc} }
type TunnelService_OpenClient interface { Send(*TunnelMessage) error; Recv() (*TunnelMessage, error); grpc.ClientStream }
func (c *tunnelServiceClient) Open(ctx context.Context, opts ...grpc.CallOption) (TunnelService_OpenClient, error) {
    desc := &grpc.StreamDesc{ServerStreams:true, ClientStreams:true}
    stream, err := c.cc.NewStream(ctx, desc, "/croupier.tunnel.v1.TunnelService/Open", opts...)
    if err != nil { return nil, err }
    return &tunnelServiceOpenClient{stream}, nil
}
type tunnelServiceOpenClient struct{ grpc.ClientStream }
func (x *tunnelServiceOpenClient) Send(m *TunnelMessage) error { return x.ClientStream.SendMsg(m) }
func (x *tunnelServiceOpenClient) Recv() (*TunnelMessage, error) { m := new(TunnelMessage); if err := x.ClientStream.RecvMsg(m); err != nil { return nil, err }; return m, nil }
