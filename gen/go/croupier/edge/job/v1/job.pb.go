package jobv1

import (
    "context"
    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

type GetJobResultRequest struct { JobId string `json:"job_id,omitempty"` }
type GetJobResultResponse struct { State string `json:"state,omitempty"`; Payload []byte `json:"payload,omitempty"`; Error string `json:"error,omitempty"` }

type JobServiceServer interface {
    GetJobResult(context.Context, *GetJobResultRequest) (*GetJobResultResponse, error)
}

type UnimplementedJobServiceServer struct{}
func (*UnimplementedJobServiceServer) GetJobResult(context.Context, *GetJobResultRequest) (*GetJobResultResponse, error) { return nil, status.Errorf(codes.Unimplemented, "method GetJobResult not implemented") }

func RegisterJobServiceServer(s *grpc.Server, srv JobServiceServer) {
    s.RegisterService(&grpc.ServiceDesc{
        ServiceName: "croupier.edge.job.v1.JobService",
        HandlerType: (*JobServiceServer)(nil),
        Methods: []grpc.MethodDesc{{ MethodName: "GetJobResult", Handler: _JobService_GetJobResult_Handler }},
        Streams: []grpc.StreamDesc{},
    }, srv)
}

func _JobService_GetJobResult_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
    in := new(GetJobResultRequest)
    if err := dec(in); err != nil { return nil, err }
    if interceptor == nil { return srv.(JobServiceServer).GetJobResult(ctx, in) }
    info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/croupier.edge.job.v1.JobService/GetJobResult"}
    handler := func(ctx context.Context, req interface{}) (interface{}, error) { return srv.(JobServiceServer).GetJobResult(ctx, req.(*GetJobResultRequest)) }
    return interceptor(ctx, in, info, handler)
}

// Client
type JobServiceClient interface { GetJobResult(ctx context.Context, in *GetJobResultRequest, opts ...grpc.CallOption) (*GetJobResultResponse, error) }
type jobServiceClient struct { cc grpc.ClientConnInterface }
func NewJobServiceClient(cc grpc.ClientConnInterface) JobServiceClient { return &jobServiceClient{cc} }
func (c *jobServiceClient) GetJobResult(ctx context.Context, in *GetJobResultRequest, opts ...grpc.CallOption) (*GetJobResultResponse, error) {
    out := new(GetJobResultResponse)
    err := c.cc.Invoke(ctx, "/croupier.edge.job.v1.JobService/GetJobResult", in, out, opts...)
    if err != nil { return nil, err }
    return out, nil
}

